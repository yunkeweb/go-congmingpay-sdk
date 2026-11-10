// Package congmingpay implements the Congmingpay basic payment API.
// Money values should be passed as strings, or json.Number for numeric fields.
package congmingpay

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxResponseSize = 4 << 20

// Params contains API field names and values exactly as specified by the gateway.
type Params map[string]any

// Response preserves all response fields, including newly added fields and their
// original JSON representation. A nil error does not imply result_code=success.
type Response map[string]json.RawMessage

// String returns a string field, or an empty string if it is absent or not a string.
func (r Response) String(key string) string { var s string; _ = json.Unmarshal(r[key], &s); return s }

// Client is safe for concurrent use provided its HTTP transport is safe for concurrent use.
type Client struct {
	baseURL        *url.URL
	programID, key string
	httpClient     *http.Client
}

// NewClient requires an HTTPS gateway origin, application ID and key. Supply a
// custom HTTP client for timeouts or a custom transport; redirects are rejected.
func NewClient(baseURL, programID, key string, hc *http.Client) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil || u == nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, errors.New("congmingpay: base URL must be an HTTPS origin without path, credentials, query or fragment")
	}
	if programID == "" || key == "" {
		return nil, errors.New("congmingpay: program ID and key are required")
	}
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	clientCopy := *hc
	clientCopy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Client{baseURL: u, programID: programID, key: key, httpClient: &clientCopy}, nil
}

// Do sends a signed request to an /api/ endpoint. It copies params and never
// mutates the caller's map. program_id and sign are managed by the client.
// API-level statuses, including processing, are left to the caller to interpret.
func (c *Client) Do(ctx context.Context, path string, params Params) (Response, error) {
	if c == nil {
		return nil, errors.New("congmingpay: nil client")
	}
	p, err := url.Parse(path)
	if err != nil || p == nil || p.IsAbs() || p.Host != "" || p.RawQuery != "" || p.Fragment != "" || p.RawPath != "" || !strings.HasPrefix(p.Path, "/api/") || strings.Contains(p.Path, "..") || strings.Contains(p.Path, "\\") {
		return nil, errors.New("congmingpay: invalid API path")
	}
	data := make(Params, len(params)+2)
	for k, v := range params {
		if k == "program_id" || k == "sign" {
			return nil, fmt.Errorf("congmingpay: %s is managed by the client", k)
		}
		data[k] = v
	}
	data["program_id"] = c.programID
	sign, err := Sign(data, c.key)
	if err != nil {
		return nil, err
	}
	data["sign"] = sign
	body, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("congmingpay: encode request: %w", err)
	}
	target := *c.baseURL
	target.Path = p.Path
	target.RawPath = ""
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target.String(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("congmingpay: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("congmingpay: request failed: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, &HTTPError{StatusCode: res.StatusCode}
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, maxResponseSize+1))
	if err != nil {
		return nil, fmt.Errorf("congmingpay: read response: %w", err)
	}
	if len(b) > maxResponseSize {
		return nil, errors.New("congmingpay: response exceeds 4 MiB")
	}
	var result Response
	if err := json.Unmarshal(b, &result); err != nil || result == nil {
		return nil, errors.New("congmingpay: response is not a JSON object")
	}
	if _, ok := result["result_code"]; !ok {
		return nil, errors.New("congmingpay: response has no result_code")
	}
	return result, nil
}

// HTTPError reports a non-2xx status without exposing potentially sensitive body data.
type HTTPError struct{ StatusCode int }

func (e *HTTPError) Error() string { return fmt.Sprintf("congmingpay: HTTP status %d", e.StatusCode) }
