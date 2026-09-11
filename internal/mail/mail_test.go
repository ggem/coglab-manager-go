package mail

import (
	"context"
	"strings"
	"testing"
)

// TestSMTPSender_Send_ConnectionError confirms Send surfaces a real
// connection failure as an error rather than swallowing it -- exercising
// the actual transmission would need a fake SMTP server, out of
// proportion for a thin wrapper around net/smtp.SendMail; internal/
// reminders' own tests exercise the interesting logic against
// mailfake.Sender instead, and TestSMTPSender_compose below covers the
// one piece of real logic this package has of its own: how the message
// bytes handed to smtp.SendMail are built.
func TestSMTPSender_Send_ConnectionError(t *testing.T) {
	s := NewSMTPSender("127.0.0.1:1", "sender@example.edu", nil)

	err := s.Send(context.Background(), Message{To: "recipient@example.edu", Subject: "Test", Body: "body"})

	if err == nil {
		t.Fatal("Send with an unreachable address returned nil error, want a connection error")
	}
}

// TestSMTPSender_compose guards against a real bug this caught live: the
// composed message had no From: header at all, only smtp.SendMail's
// envelope-only `from` parameter -- invisible to the recipient and to
// DMARC's From-header alignment check. A relay receiving a headerless
// message is free to substitute its own default identity instead, which
// is exactly what happened (Gmail substituted an account-level default
// whose domain's DMARC policy then rejected the message outright,
// regardless of what SMTP_FROM was actually configured to).
func TestSMTPSender_compose(t *testing.T) {
	s := NewSMTPSender("smtp.example.edu:587", "sender@example.edu", nil)

	got := string(s.compose(Message{To: "recipient@example.edu", Subject: "Test Subject", Body: "the body"}))

	if !strings.HasPrefix(got, "From: sender@example.edu\r\n") {
		t.Errorf("compose() = %q, want it to start with an explicit From: header matching the configured sender", got)
	}
	if !strings.Contains(got, "To: recipient@example.edu\r\n") {
		t.Errorf("compose() = %q, want a To: header", got)
	}
	if !strings.Contains(got, "Subject: Test Subject\r\n") {
		t.Errorf("compose() = %q, want a Subject: header", got)
	}
	if !strings.HasSuffix(got, "\r\n\r\nthe body") {
		t.Errorf("compose() = %q, want the body after a blank-line header/body separator", got)
	}
}
