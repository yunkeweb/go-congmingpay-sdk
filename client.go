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

// Client is safe for concurrent use provided its HTTP transport is safe for concurrent use.
type Client struct {
	baseURL                *url.URL
	programID, shopID, key string
	httpClient             *http.Client
	logHook                LogHook
	logRedactFields        map[string]struct{}
}

// Config contains credentials for one merchant.
type Config struct {
	BaseURL    string
	ProgramID  string
	ShopID     string
	Key        string
	HTTPClient *http.Client
	// LogRedactFields contains payload field names whose values are replaced
	// with [REDACTED] in logs. Matching ignores case, underscores, and hyphens.
	// The same rule applies to query parameter names in HTTP(S) URL strings.
	// By default, no payload fields are redacted.
	LogRedactFields []string
}

// NewClient constructs an immutable merchant client. Redirects are rejected.
func NewClient(config Config) (*Client, error) {
	u, err := url.Parse(config.BaseURL)
	if err != nil || u == nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, errors.New("congmingpay: base URL must be an HTTPS origin without path, credentials, query or fragment")
	}
	if config.ProgramID == "" || config.ShopID == "" || config.Key == "" {
		return nil, errors.New("congmingpay: program ID, shop ID and key are required")
	}
	hc := config.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	clientCopy := *hc
	clientCopy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	redactFields := make(map[string]struct{}, len(config.LogRedactFields))
	for _, field := range config.LogRedactFields {
		if normalized := normalizeLogField(field); normalized != "" {
			redactFields[normalized] = struct{}{}
		}
	}
	return &Client{baseURL: u, programID: config.ProgramID, shopID: config.ShopID, key: config.Key, httpClient: &clientCopy, logHook: defaultLogHook, logRedactFields: redactFields}, nil
}

// Do sends a signed request to an /api/ endpoint. It copies params and never
// mutates the caller's map. program_id, shop_id and sign are managed by the client.
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
		if k == "program_id" || k == "shop_id" || k == "sign" {
			return nil, fmt.Errorf("congmingpay: %s is managed by the client", k)
		}
		if v != nil {
			data[k] = v
		}
	}
	data["program_id"] = c.programID
	data["shop_id"] = c.shopID
	// Freeze values once so custom marshalers cannot change signed values.
	var frozen bytes.Buffer
	enc := json.NewEncoder(&frozen)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(data); err != nil {
		return nil, fmt.Errorf("congmingpay: encode request: %w", err)
	}
	var snapshot map[string]json.RawMessage
	if err := json.Unmarshal(frozen.Bytes(), &snapshot); err != nil {
		return nil, fmt.Errorf("congmingpay: decode request: %w", err)
	}
	data = make(Params, len(snapshot)+1)
	for k, v := range snapshot {
		if string(v) != "null" {
			data[k] = v
		}
	}
	sign, err := Sign(data, c.key)
	if err != nil {
		return nil, err
	}
	data["sign"] = sign
	var bodyBuffer bytes.Buffer
	bodyEncoder := json.NewEncoder(&bodyBuffer)
	bodyEncoder.SetEscapeHTML(false)
	if err := bodyEncoder.Encode(data); err != nil {
		return nil, fmt.Errorf("congmingpay: encode request: %w", err)
	}
	body := bytes.TrimSuffix(bodyBuffer.Bytes(), []byte("\n"))
	target := *c.baseURL
	target.Path = p.Path
	target.RawPath = ""
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target.String(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("congmingpay: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	c.logPayload(ctx, "request", p.Path, 0, body)
	res, err := c.httpClient.Do(req)
	if err != nil {
		c.logFailure(ctx, p.Path, "request failed")
		return nil, fmt.Errorf("congmingpay: request failed: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		b, readErr := io.ReadAll(io.LimitReader(res.Body, maxResponseSize+1))
		truncated := len(b) > maxResponseSize
		if truncated {
			b = b[:maxResponseSize]
		}
		// Log before exposing the diagnostic bytes, using an independent payload.
		if readErr == nil && !truncated {
			c.logPayload(ctx, "response", p.Path, res.StatusCode, b)
		} else {
			c.logFailure(ctx, p.Path, "read HTTP error response failed or too large")
		}
		return nil, &HTTPError{
			StatusCode: res.StatusCode, Path: p.Path, Body: b,
			BodyTruncated: truncated, RequestID: res.Header.Get("X-Request-ID"),
			RetryAfter: res.Header.Get("Retry-After"), Cause: readErr,
		}
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, maxResponseSize+1))
	if err != nil {
		c.logFailure(ctx, p.Path, "read response failed")
		return nil, fmt.Errorf("congmingpay: read response: %w", err)
	}
	if len(b) > maxResponseSize {
		c.logFailure(ctx, p.Path, "response too large")
		return nil, errors.New("congmingpay: response exceeds 4 MiB")
	}
	c.logPayload(ctx, "response", p.Path, res.StatusCode, b)
	var result Response
	if err := json.Unmarshal(b, &result); err != nil || result == nil {
		return nil, errors.New("congmingpay: response is not a JSON object")
	}
	if _, ok := result["result_code"]; !ok {
		return nil, errors.New("congmingpay: response has no result_code")
	}
	return result, nil
}

// HTTPError reports a non-2xx status. Error omits the body and diagnostic headers;
// callers can explicitly inspect Body, which may contain sensitive data.
type HTTPError struct {
	StatusCode int
	Path       string
	// Body contains at most 4 MiB of the original response, even with logging disabled.
	Body []byte
	// BodyTruncated indicates the size limit was exceeded, not a read failure.
	BodyTruncated bool
	// RequestID is the X-Request-ID header, if present.
	RequestID string
	// RetryAfter is the unparsed Retry-After header. The SDK does not retry.
	RetryAfter string
	// Cause is a response body read error, if any. Body may contain partial data.
	Cause error
}

func (e *HTTPError) Error() string {
	if e.Path == "" {
		return fmt.Sprintf("congmingpay: HTTP status %d", e.StatusCode)
	}
	return fmt.Sprintf("congmingpay: HTTP status %d for %s", e.StatusCode, e.Path)
}

// Unwrap preserves errors.Is and errors.As for response body read errors.
func (e *HTTPError) Unwrap() error { return e.Cause }
