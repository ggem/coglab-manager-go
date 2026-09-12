// Package mail sends plain-text email. It knows nothing about who's being
// emailed or why -- the domain packages that decide that (internal/
// reminders) depend on the Sender interface here, not on SMTPSender
// directly, the same seam internal/auth draws between "how we verify an
// identity" and the LocalAuthenticator interface a caller depends on.
package mail

import (
	"context"
	"fmt"
	"mime"
	"net/smtp"
	"strings"
)

// Message is a plain-text email -- matching the legacy app's own mail,
// which was plain text throughout (no HTML templates to port).
type Message struct {
	To      string
	Subject string
	Body    string
}

// Sender sends a Message, or reports why it couldn't.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}

// SMTPSender sends mail through an SMTP relay via the standard library's
// net/smtp -- no new dependency for what's a low-volume internal mailer.
// Known limitation: net/smtp has no context support, so Send can't be
// canceled mid-call once dialing starts; acceptable here, not fixed.
type SMTPSender struct {
	addr string // host:port
	from string
	auth smtp.Auth // nil if the relay needs no authentication
}

// NewSMTPSender builds a sender for the relay at addr, sending as from.
// auth may be nil for a relay that doesn't require authentication (e.g.
// a local Postfix/relay on the same host, matching legacy's own setup).
func NewSMTPSender(addr, from string, auth smtp.Auth) *SMTPSender {
	return &SMTPSender{addr: addr, from: from, auth: auth}
}

func (s *SMTPSender) Send(ctx context.Context, msg Message) error {
	body, err := s.compose(msg)
	if err != nil {
		return err
	}
	return smtp.SendMail(s.addr, s.auth, s.from, []string{msg.To}, body)
}

// compose builds the RFC 5322 message smtp.SendMail transmits.
// s.from is passed to smtp.SendMail separately too, but only as the
// envelope MAIL FROM -- that's invisible to the recipient and to SPF/
// DKIM/DMARC's From-header alignment check, which looks at the visible
// header set here. A message with no From: header at all (this
// function's own prior form) leaves the relay to fill one in however it
// sees fit -- caught live when Gmail substituted its own account-level
// default identity instead of s.from, and that substituted domain's
// DMARC policy rejected the message outright.
//
// Declares UTF-8 explicitly (participant/guardian names routinely need
// more than US-ASCII, RFC 2045's default charset) -- the Subject header
// itself needs RFC 2047 encoding to carry non-ASCII text at all (a raw
// UTF-8 byte in a header technically isn't legal RFC 5322), which
// mime.QEncoding.Encode handles, returning the input unchanged when it's
// already plain ASCII. Content-Transfer-Encoding: 8bit is a factual
// declaration, not a request -- net/smtp.SendMail doesn't negotiate
// ESMTP 8BITMIME, but every relay this app is likely to run against
// tolerates raw 8-bit UTF-8 bodies in practice; quoted-printable/base64
// body encoding would be the fully strict alternative if that ever stops
// being true.
func (s *SMTPSender) compose(msg Message) ([]byte, error) {
	// RFC 5322 headers are single lines -- an embedded CR or LF in a
	// value placed directly into one (unlike the body, header values
	// here are never user-facing free text meant to wrap) would let that
	// value inject additional header lines, or, past a blank line, a
	// forged message body, into what's transmitted.
	for name, v := range map[string]string{"From": s.from, "To": msg.To, "Subject": msg.Subject} {
		if strings.ContainsAny(v, "\r\n") {
			return nil, fmt.Errorf("invalid %s header value: contains a line break", name)
		}
	}
	subject := mime.QEncoding.Encode("UTF-8", msg.Subject)
	return []byte(fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n%s",
		s.from, msg.To, subject, msg.Body,
	)), nil
}
