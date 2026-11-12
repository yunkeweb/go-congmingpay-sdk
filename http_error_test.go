package congmingpay

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestHTTPErrorDiagnostics(t *testing.T) {
	for _, body := range []string{`{"error":"rate limited","token":"secret"}`, "upstream unavailable: secret"} {
		t.Run(body[:8], func(t *testing.T) {
			var calls atomic.Int32
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.Header().Set("X-Request-ID", "secret-request-id")
				w.Header().Set("Retry-After", "Wed, 18 Nov 2026 00:00:00 GMT")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = io.WriteString(w, body)
			}))
			defer srv.Close()
			client, err := NewClient(Config{BaseURL: srv.URL, ProgramID: "app", ShopID: "shop", Key: "key", HTTPClient: srv.Client()})
			if err != nil {
				t.Fatal(err)
			}
			result, err := client.WithLogHook(nil).Do(context.Background(), "/api/query.do", nil)
			var httpErr *HTTPError
			if result != nil || !errors.As(err, &httpErr) {
				t.Fatalf("expected HTTPError: result=%v err=%v", result, err)
			}
			if httpErr.StatusCode != 429 || httpErr.Path != "/api/query.do" || string(httpErr.Body) != body || httpErr.RequestID != "secret-request-id" || httpErr.RetryAfter != "Wed, 18 Nov 2026 00:00:00 GMT" || httpErr.BodyTruncated || httpErr.Cause != nil || calls.Load() != 1 {
				t.Fatalf("incorrect diagnostics: %+v calls=%d", httpErr, calls.Load())
			}
			if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), srv.URL) || !strings.Contains(err.Error(), "/api/query.do") {
				t.Fatalf("unexpected error text: %v", err)
			}
		})
	}
}

type errorTestTransport func(*http.Request) (*http.Response, error)

func (f errorTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type partialErrorBody struct {
	read   bool
	closed bool
	cause  error
}

func (b *partialErrorBody) Read(p []byte) (int, error) {
	if b.read {
		return 0, b.cause
	}
	b.read = true
	return copy(p, "partial-secret"), b.cause
}

func (b *partialErrorBody) Close() error { b.closed = true; return nil }

func TestHTTPErrorBoundedAndPartialBody(t *testing.T) {
	for _, cause := range []error{io.ErrUnexpectedEOF, context.Canceled, context.DeadlineExceeded} {
		body := &partialErrorBody{cause: cause}
		client, err := NewClient(Config{
			BaseURL: "https://example.com", ProgramID: "app", ShopID: "shop", Key: "key",
			HTTPClient: &http.Client{Transport: errorTestTransport(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 503, Body: body, Header: make(http.Header)}, nil
			})},
		})
		if err != nil {
			t.Fatal(err)
		}
		_, err = client.WithLogHook(nil).Do(context.Background(), "/api/query.do", nil)
		var httpErr *HTTPError
		if !errors.As(err, &httpErr) || !errors.Is(err, cause) {
			t.Fatalf("lost error chain: %v", err)
		}
		if string(httpErr.Body) != "partial-secret" || httpErr.BodyTruncated || !body.closed || strings.Contains(err.Error(), "partial-secret") || strings.Contains(err.Error(), cause.Error()) {
			t.Fatalf("incorrect partial read diagnostics: %+v", httpErr)
		}
	}
	for _, size := range []int{maxResponseSize, maxResponseSize + 1} {
		body := strings.Repeat("x", size)
		client, err := NewClient(Config{
			BaseURL: "https://example.com", ProgramID: "app", ShopID: "shop", Key: "key",
			HTTPClient: &http.Client{Transport: errorTestTransport(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 502, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})},
		})
		if err != nil {
			t.Fatal(err)
		}
		_, err = client.WithLogHook(nil).Do(context.Background(), "/api/query.do", nil)
		var httpErr *HTTPError
		if !errors.As(err, &httpErr) || len(httpErr.Body) != maxResponseSize || httpErr.BodyTruncated != (size > maxResponseSize) || httpErr.Cause != nil {
			t.Fatalf("incorrect body limit for size %d: %v", size, err)
		}
	}
}

func TestTransportContextErrors(t *testing.T) {
	for _, cause := range []error{context.Canceled, context.DeadlineExceeded} {
		client, err := NewClient(Config{
			BaseURL: "https://example.com", ProgramID: "app", ShopID: "shop", Key: "key",
			HTTPClient: &http.Client{Transport: errorTestTransport(func(*http.Request) (*http.Response, error) {
				return nil, cause
			})},
		})
		if err != nil {
			t.Fatal(err)
		}
		_, err = client.WithLogHook(nil).Do(context.Background(), "/api/query.do", nil)
		var httpErr *HTTPError
		if !errors.Is(err, cause) || errors.As(err, &httpErr) {
			t.Fatalf("incorrect transport error: %v", err)
		}
	}
}

func TestHTTPErrorLogPayloadIsolation(t *testing.T) {
	const rawBody = `{"error":"rate limited","token":"response-secret"}`
	client, err := NewClient(Config{
		BaseURL: "https://example.com", ProgramID: "app", ShopID: "shop", Key: "key",
		LogRedactFields: []string{"token"},
		HTTPClient: &http.Client{Transport: errorTestTransport(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 429, Body: io.NopCloser(strings.NewReader(rawBody)), Header: make(http.Header)}, nil
		})},
	})
	if err != nil {
		t.Fatal(err)
	}
	logged := false
	client = client.WithLogHook(func(_ context.Context, event LogEvent) {
		if event.Event != "response" {
			return
		}
		logged = true
		payload := event.Payload.(map[string]any)
		if payload["token"] != "[REDACTED]" {
			t.Fatalf("configured field not redacted: %v", payload)
		}
		payload["error"] = "changed by hook"
	})
	_, err = client.Do(context.Background(), "/api/query.do", nil)
	var httpErr *HTTPError
	if !logged || !errors.As(err, &httpErr) || string(httpErr.Body) != rawBody {
		t.Fatalf("log hook changed HTTP diagnostics: %v", err)
	}
}
