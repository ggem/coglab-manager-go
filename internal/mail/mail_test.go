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

	got, err := s.compose(Message{To: "recipient@example.edu", Subject: "Test Subject", Body: "the body"})
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	gotStr := string(got)

	if !strings.HasPrefix(gotStr, "From: sender@example.edu\r\n") {
		t.Errorf("compose() = %q, want it to start with an explicit From: header matching the configured sender", gotStr)
	}
	if !strings.Contains(gotStr, "To: recipient@example.edu\r\n") {
		t.Errorf("compose() = %q, want a To: header", gotStr)
	}
	if !strings.Contains(gotStr, "Subject: Test Subject\r\n") {
		t.Errorf("compose() = %q, want a Subject: header", gotStr)
	}
	if !strings.HasSuffix(gotStr, "\r\n\r\nthe body") {
		t.Errorf("compose() = %q, want the body after a blank-line header/body separator", gotStr)
	}
}

// TestSMTPSender_compose_UTF8Body guards against a real gap: participant
// and guardian names routinely need more than US-ASCII, RFC 2045's
// default charset for a message with no declared charset -- without an
// explicit declaration, a mail client is entitled to assume US-ASCII and
// mangle anything outside it.
func TestSMTPSender_compose_UTF8Body(t *testing.T) {
	s := NewSMTPSender("smtp.example.edu:587", "sender@example.edu", nil)

	got, err := s.compose(Message{To: "recipient@example.edu", Subject: "Test", Body: "Hi François, ...\nBody continues"})
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	gotStr := string(got)

	if !strings.Contains(gotStr, "MIME-Version: 1.0\r\n") {
		t.Errorf("compose() = %q, want a MIME-Version header", gotStr)
	}
	if !strings.Contains(gotStr, "Content-Type: text/plain; charset=UTF-8\r\n") {
		t.Errorf("compose() = %q, want an explicit UTF-8 Content-Type", gotStr)
	}
	if !strings.HasSuffix(gotStr, "\r\n\r\nHi François, ...\nBody continues") {
		t.Errorf("compose() = %q, want the UTF-8 body passed through unescaped", gotStr)
	}
}

// TestSMTPSender_compose_UTF8Subject guards the same gap for the Subject
// header specifically: unlike the body, a raw UTF-8 byte in a header
// isn't legal RFC 5322 at all, so non-ASCII subject text (an experiment
// name with an accented character, say) needs RFC 2047 encoding rather
// than just a charset declaration.
func TestSMTPSender_compose_UTF8Subject(t *testing.T) {
	s := NewSMTPSender("smtp.example.edu:587", "sender@example.edu", nil)

	got, err := s.compose(Message{To: "recipient@example.edu", Subject: "Étude Study", Body: "body"})
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	gotStr := string(got)

	if strings.Contains(gotStr, "Subject: Étude Study\r\n") {
		t.Errorf("compose() = %q, want the Subject RFC 2047-encoded, not raw UTF-8 bytes in the header", gotStr)
	}
	if !strings.Contains(gotStr, "Subject: =?utf-8?q?") && !strings.Contains(gotStr, "Subject: =?UTF-8?q?") {
		t.Errorf("compose() = %q, want an RFC 2047 encoded-word Subject", gotStr)
	}
}

// TestSMTPSender_compose_RejectsHeaderInjection guards against a real
// class of bug: a CR or LF embedded in a value placed directly into a
// header (unlike the compose_ConnectionError test's from an unreachable
// address) lets that value inject additional header lines -- or, past a
// blank line, a forged message body -- into what's actually transmitted.
// msg.To in particular traces back to a guardian's stored email address,
// which isn't guaranteed to be free of stray control characters.
func TestSMTPSender_compose_RejectsHeaderInjection(t *testing.T) {
	s := NewSMTPSender("smtp.example.edu:587", "sender@example.edu", nil)

	_, err := s.compose(Message{
		To:      "victim@example.edu\r\nBcc: attacker@evil.example",
		Subject: "Test",
		Body:    "body",
	})

	if err == nil {
		t.Fatal("compose returned nil error for a To value containing a header injection attempt, want an error")
	}
}
