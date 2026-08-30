// Package mailfake provides a fake mail.Sender for tests, so packages
// that depend on mail.Sender can be tested without a real SMTP relay --
// mirrors internal/db/dbfake's role for db.Querier, sized down to match
// mail.Sender's single method rather than that package's function-field-
// per-method shape.
package mailfake

import (
	"context"
	"sync"

	"github.com/ggem/coglab-manager-go/internal/mail"
)

// Sender records every message passed to Send, optionally failing with
// Err instead. Safe for concurrent use, since the scheduler this backs
// in tests may send from more than one goroutine.
type Sender struct {
	mu   sync.Mutex
	Sent []mail.Message
	Err  error
}

func (s *Sender) Send(ctx context.Context, msg mail.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	s.Sent = append(s.Sent, msg)
	return nil
}

// Messages returns a snapshot of every message sent so far.
func (s *Sender) Messages() []mail.Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]mail.Message(nil), s.Sent...)
}
