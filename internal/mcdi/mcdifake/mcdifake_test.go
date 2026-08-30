package mcdifake

import (
	"context"
	"errors"
	"testing"

	"github.com/ggem/coglab-manager-go/internal/mcdi"
)

func TestClient_RequestSurvey_Records(t *testing.T) {
	c := &Client{}

	if err := c.RequestSurvey(context.Background(), mcdi.Request{ChildName: "Kid A", DatabaseID: 1}); err != nil {
		t.Fatalf("RequestSurvey: %v", err)
	}
	if err := c.RequestSurvey(context.Background(), mcdi.Request{ChildName: "Kid B", DatabaseID: 2}); err != nil {
		t.Fatalf("RequestSurvey: %v", err)
	}

	got := c.Sent()
	if len(got) != 2 || got[0].ChildName != "Kid A" || got[1].ChildName != "Kid B" {
		t.Errorf("Sent() = %+v, want two requests for Kid A then Kid B", got)
	}
}

func TestClient_RequestSurvey_ConfiguredError(t *testing.T) {
	wantErr := errors.New("daxlabbase down")
	c := &Client{Err: wantErr}

	err := c.RequestSurvey(context.Background(), mcdi.Request{DatabaseID: 1})

	if !errors.Is(err, wantErr) {
		t.Errorf("err = %v, want %v", err, wantErr)
	}
	if len(c.Sent()) != 0 {
		t.Errorf("Sent() = %+v, want none recorded when RequestSurvey fails", c.Sent())
	}
}
