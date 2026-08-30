// Package mcdi requests a MacArthur-Bates CDI language-development
// survey from daxlabbase/cdibase, the sibling service this app has
// always depended on for that workflow. It knows nothing about children,
// families, or guardians -- the HTTP handler that decides who to
// request a survey for depends on the Client interface here, the same
// seam internal/mail draws between "how to send" and who/why.
package mcdi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Request is one survey request. Birthday is "YYYY-MM-DD"; the exact
// format the live service expects isn't fully knowable from the legacy
// caller's source alone and should be confirmed against it directly.
type Request struct {
	ChildName   string
	ParentEmail string
	Gender      string // "male", "female", or "other" -- see GenderFor
	Birthday    string
	DatabaseID  int64
}

// Client requests a survey, or reports why it couldn't. Unlike legacy's
// caller (which captured the API's response and never inspected it,
// always reporting success to staff), a real Client implementation
// checks the response and returns a real error on failure.
type Client interface {
	RequestSurvey(ctx context.Context, req Request) error
}

// GenderFor maps this app's children.sex ('unknown'|'male'|'female') to
// the three-bucket gender daxlabbase/cdibase's own percentile norm data
// uses (male/female/other) -- "unknown" -> "other" is a direct match to
// that third bucket, not an approximation.
func GenderFor(sex string) string {
	switch sex {
	case "male", "female":
		return sex
	default:
		return "other"
	}
}

// mcdiFormType is the one CDI form legacy ever requested in 10+ years of
// real use, out of several the underlying tool supports -- kept
// hardcoded to match, not exposed as a choice (see the M9 plan's Context
// section for why).
const mcdiFormType = "fullenglishmcdi"

// APIClient is the real Client, talking to a daxlabbase/cdibase
// instance over HTTPS.
type APIClient struct {
	baseURL    string
	apiKey     string
	typeParam  string // query param name for the form type -- "cdi_type" on the current tool, "mcdi_type" on the older one it renamed from
	httpClient *http.Client
}

// NewAPIClient builds a Client for the daxlabbase/cdibase instance at
// baseURL. typeParam names the form-type query parameter; pass "" for
// the current tool's "cdi_type" default.
func NewAPIClient(baseURL, apiKey, typeParam string) *APIClient {
	if typeParam == "" {
		typeParam = "cdi_type"
	}
	return &APIClient{
		baseURL:   baseURL,
		apiKey:    apiKey,
		typeParam: typeParam,
		// A real outbound call to a third-party service, unlike
		// net/smtp's SendMail (no client to configure) -- worth a
		// timeout so one unresponsive request can't hang indefinitely.
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// apiResponse is the JSON body daxlabbase/cdibase returns -- legacy's
// caller captured this and never looked at it, always reporting success
// regardless. This Client actually checks it.
type apiResponse struct {
	Msg   string `json:"msg"`
	Error string `json:"error"`
}

func (c *APIClient) RequestSurvey(ctx context.Context, req Request) error {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return fmt.Errorf("invalid mcdi base url: %w", err)
	}
	values := url.Values{
		"api_key":      {c.apiKey},
		"child_name":   {req.ChildName},
		"parent_email": {req.ParentEmail},
		"format":       {"standard"},
		"database_id":  {strconv.FormatInt(req.DatabaseID, 10)},
		"gender":       {req.Gender},
		"birthday":     {req.Birthday},
	}
	values.Set(c.typeParam, mcdiFormType)
	u.RawQuery = values.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), nil)
	if err != nil {
		return fmt.Errorf("build mcdi request: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("send mcdi request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read mcdi response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("mcdi request failed: status %d: %s", resp.StatusCode, body)
	}

	var parsed apiResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return fmt.Errorf("parse mcdi response: %w", err)
	}
	if parsed.Error != "" {
		return fmt.Errorf("mcdi request failed: %s", parsed.Error)
	}
	if parsed.Msg != "success" {
		return fmt.Errorf("mcdi request did not report success: %q", parsed.Msg)
	}
	return nil
}
