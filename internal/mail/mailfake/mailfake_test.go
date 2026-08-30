package mailfake

import (
	"context"
	"errors"
	"testing"

	"github.com/ggem/coglab-manager-go/internal/mail"
)

func TestSender_Send_Records(t *testing.T) {
	s := &Sender{}

	if err := s.Send(context.Background(), mail.Message{To: "a@example.edu", Subject: "Hi", Body: "body"}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if err := s.Send(context.Background(), mail.Message{To: "b@example.edu", Subject: "Hi again", Body: "body2"}); err != nil {
		t.Fatalf("Send: %v", err)
	}

	got := s.Messages()
	if len(got) != 2 || got[0].To != "a@example.edu" || got[1].To != "b@example.edu" {
		t.Errorf("Messages() = %+v, want two messages to a@example.edu then b@example.edu", got)
	}
}

func TestSender_Send_ConfiguredError(t *testing.T) {
	wantErr := errors.New("smtp relay down")
	s := &Sender{Err: wantErr}

	err := s.Send(context.Background(), mail.Message{To: "a@example.edu"})

	if !errors.Is(err, wantErr) {
		t.Errorf("Send err = %v, want %v", err, wantErr)
	}
	if len(s.Messages()) != 0 {
		t.Errorf("Messages() = %+v, want none recorded when Send fails", s.Messages())
	}
}
