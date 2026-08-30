// Package mail sends plain-text email. It knows nothing about who's being
// emailed or why -- the domain packages that decide that (internal/
// reminders) depend on the Sender interface here, not on SMTPSender
// directly, the same seam internal/auth draws between "how we verify an
// identity" and the LocalAuthenticator interface a caller depends on.
package mail

import (
	"context"
	"fmt"
	"net/smtp"
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
	body := fmt.Sprintf("To: %s\r\nSubject: %s\r\n\r\n%s", msg.To, msg.Subject, msg.Body)
	return smtp.SendMail(s.addr, s.auth, s.from, []string{msg.To}, []byte(body))
}
