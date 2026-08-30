package mail

import (
	"context"
	"testing"
)

// TestSMTPSender_Send_ConnectionError confirms Send surfaces a real
// connection failure as an error rather than swallowing it -- exercising
// the actual success path would need a fake SMTP server, out of
// proportion for a thin wrapper around net/smtp.SendMail; internal/
// reminders' own tests exercise the interesting logic against
// mailfake.Sender instead.
func TestSMTPSender_Send_ConnectionError(t *testing.T) {
	s := NewSMTPSender("127.0.0.1:1", "sender@example.edu", nil)

	err := s.Send(context.Background(), Message{To: "recipient@example.edu", Subject: "Test", Body: "body"})

	if err == nil {
		t.Fatal("Send with an unreachable address returned nil error, want a connection error")
	}
}
