package congmingpay

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
)

type testLogger struct {
	mu      sync.Mutex
	entries []LogEvent
}

func (l *testLogger) hook(_ context.Context, event LogEvent) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, event)
}
func (l *testLogger) snapshot() []LogEvent {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]LogEvent(nil), l.entries...)
}

func TestLogging(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/error":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"result_code":"FAIL","token":"response-secret"}`))
		case "/api/invalid":
			_, _ = w.Write([]byte("response-secret"))
		default:
			_, _ = w.Write([]byte(`{"result_code":"success","nested":{"token":"response-secret"},"data":"{\"token\":\"response-secret\"}"}`))
		}
	}))
	defer srv.Close()
	client, err := NewClient(Config{BaseURL: srv.URL, ProgramID: "app-secret", ShopID: "shop", Key: "key-secret", HTTPClient: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	logger := &testLogger{}
	logged := client.WithLogHook(logger.hook)
	params := Params{"auth_code": "request-secret", "nested": map[string]any{"token": "nested-secret"}}
	if _, err := client.Do(context.Background(), "/api/success", params); err != nil {
		t.Fatal(err)
	}
	if len(logger.snapshot()) != 0 {
		t.Fatal("logging enabled on original client")
	}
	if _, err := logged.Do(context.Background(), "/api/success", params); err != nil {
		t.Fatal(err)
	}
	if params["auth_code"] != "request-secret" || len(params) != 2 {
		t.Fatal("parameters modified")
	}
	_, err = logged.Do(context.Background(), "/api/error", nil)
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err = logged.Do(context.Background(), "/api/invalid", nil); err == nil {
		t.Fatal("expected invalid JSON error")
	}
	entries := logger.snapshot()
	if len(entries) != 6 {
		t.Fatalf("got %d entries", len(entries))
	}
	if entries[1].Event != "response" || entries[1].StatusCode != http.StatusOK || entries[3].StatusCode != http.StatusBadRequest {
		t.Fatalf("unexpected entries: %+v", entries)
	}
	b, err := json.Marshal(entries)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"request-secret", "nested-secret", "response-secret", "app-secret", "key-secret"} {
		if strings.Contains(string(b), secret) {
			t.Fatalf("leaked %q: %s", secret, b)
		}
	}
	if !strings.Contains(string(b), "success") || !strings.Contains(string(b), "[invalid JSON redacted]") {
		t.Fatalf("missing status or invalid marker: %s", b)
	}
}

func TestLoggingTransportError(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	client, err := NewClient(Config{BaseURL: srv.URL, ProgramID: "app", ShopID: "shop", Key: "key", HTTPClient: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	srv.Close()
	logger := &testLogger{}
	if _, err = client.WithLogHook(logger.hook).Do(context.Background(), "/api/test", nil); err == nil {
		t.Fatal("expected transport error")
	}
	entries := logger.snapshot()
	if len(entries) != 2 || entries[1].Event != "error" || entries[1].Error != "request failed" {
		t.Fatalf("unexpected entries: %+v", entries)
	}
}

func TestDefaultAndDisabledLogHook(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result_code":"success","token":"response-secret"}`))
	}))
	defer srv.Close()
	client, err := NewClient(Config{BaseURL: srv.URL, ProgramID: "app", ShopID: "shop", Key: "key", HTTPClient: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stdout
	os.Stdout = writer
	defer func() { os.Stdout = original; _ = reader.Close(); _ = writer.Close() }()
	_, err = client.Do(context.Background(), "/api/default", Params{"auth_code": "request-secret"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.WithLogHook(nil).Do(context.Background(), "/api/disabled", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = original
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	result := string(output)
	for _, expected := range []string{`"event":"request"`, `"event":"response"`, `"status_code":200`, "[REDACTED]"} {
		if !strings.Contains(result, expected) {
			t.Fatalf("missing %q: %s", expected, result)
		}
	}
	for _, secret := range []string{"request-secret", "response-secret", "/api/disabled"} {
		if strings.Contains(result, secret) {
			t.Fatalf("unexpected %q: %s", secret, result)
		}
	}
}
