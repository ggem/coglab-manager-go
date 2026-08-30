// Package mcdifake provides a fake mcdi.Client for tests, so packages
// that depend on mcdi.Client can be tested without a real daxlabbase/
// cdibase instance -- mirrors internal/mail/mailfake's role for
// mail.Sender.
package mcdifake

import (
	"context"
	"sync"

	"github.com/ggem/coglab-manager-go/internal/mcdi"
)

// Client records every request passed to RequestSurvey, optionally
// failing with Err instead. Safe for concurrent use.
type Client struct {
	mu       sync.Mutex
	Requests []mcdi.Request
	Err      error
}

func (c *Client) RequestSurvey(ctx context.Context, req mcdi.Request) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Err != nil {
		return c.Err
	}
	c.Requests = append(c.Requests, req)
	return nil
}

// Sent returns a snapshot of every request sent so far.
func (c *Client) Sent() []mcdi.Request {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]mcdi.Request(nil), c.Requests...)
}
