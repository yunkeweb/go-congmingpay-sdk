package congmingpay

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
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
			_, _ = w.Write([]byte(`{"result_code":"success","order_id":"order-123","order_status":"paid","amount":12.5,"note":"paid order","nested":{"token":"response-secret","shop_name":"Example Shop"},"data":"{\"token\":\"response-secret\",\"order_id\":\"order-embedded\"}"}`))
		}
	}))
	defer srv.Close()
	client, err := NewClient(Config{BaseURL: srv.URL, ProgramID: "app-id", ShopID: "shop", Key: "key-secret", HTTPClient: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	logger := &testLogger{}
	logged := client.WithLogHook(logger.hook)
	params := Params{"order_id": "order-123", "order_status": "paid", "amount": 12.5, "auth_code": "request-secret", "nested": map[string]any{"token": "nested-secret", "shop_name": "Example Shop"}}
	if _, err := client.Do(context.Background(), "/api/success", params); err != nil {
		t.Fatal(err)
	}
	if len(logger.snapshot()) != 0 {
		t.Fatal("logging enabled on original client")
	}
	if _, err := logged.Do(context.Background(), "/api/success", params); err != nil {
		t.Fatal(err)
	}
	if params["auth_code"] != "request-secret" || len(params) != 5 {
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
	request := entries[0].Payload.(map[string]any)
	if request["sign"] != "[REDACTED]" || request["auth_code"] != "[REDACTED]" {
		t.Fatalf("request credentials not redacted: %+v", request)
	}
	b, err := json.Marshal(entries)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"request-secret", "nested-secret", "response-secret", "key-secret"} {
		if strings.Contains(string(b), secret) {
			t.Fatalf("leaked %q: %s", secret, b)
		}
	}
	for _, expected := range []string{"success", "order-123", "order-embedded", "paid", "12.5", "paid order", "Example Shop", "app-id", "shop", "[REDACTED]", "[invalid JSON redacted]"} {
		if !strings.Contains(string(b), expected) {
			t.Fatalf("missing %q: %s", expected, b)
		}
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
		_, _ = w.Write([]byte(`{"result_code":"success","order_id":"order-default","order_status":"paid","token":"response-secret"}`))
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
	_, err = client.Do(context.Background(), "/api/default", Params{"order_id": "order-default", "auth_code": "request-secret"})
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
	for _, expected := range []string{"order-default", "paid", "success"} {
		if !strings.Contains(result, expected) {
			t.Fatalf("missing ordinary value %q: %s", expected, result)
		}
	}
	for _, secret := range []string{"request-secret", "response-secret", "/api/disabled"} {
		if strings.Contains(result, secret) {
			t.Fatalf("unexpected %q: %s", secret, result)
		}
	}
}

func TestLogSensitiveFields(t *testing.T) {
	fields := []string{
		"key", "api_key", "privateKey", "secret_key", "short_key", "appSecret", "client_secret",
		"sign", "SIGN", "signature", "paySign", "password", "passwd",
		"token", "access_token", "refreshToken", "id-token", "Authorization", "credential",
		"auth_code", "authCode", "code", "openid", "sub_openid", "unionId",
		"phone", "phone_number", "mobile", "mobileNumber", "id_card", "idCardNo",
		"bank_card", "bank_card_no", "card_no", "cardNumber", "cvv", "cvc",
	}
	for _, field := range fields {
		t.Run(field, func(t *testing.T) {
			for _, value := range []any{"sensitive", json.Number("123456"), map[string]any{"value": "sensitive"}, []any{"sensitive"}} {
				if got := redactLogValue(value, field); got != "[REDACTED]" {
					t.Fatalf("sensitive field %q not redacted: %v", field, got)
				}
			}
		})
	}
}

func TestLogBusinessFieldsAndNestedValues(t *testing.T) {
	input := map[string]any{
		"order_id": "order-123", "program_id": "app", "shop_id": "shop", "shopId": "shop",
		"order_status": "paid", "result_code": "CUSTOM_STATUS", "error_msg": "order not found",
		"money": json.Number("12.30"), "enabled": true, "empty": "", "missing": nil,
		"design": "normal", "monkey": "normal", "token_count": json.Number("2"),
		"items": []any{map[string]any{"order_id": "child", "accessToken": "secret"}, "ordinary", false, nil},
		"sign":  map[string]any{"value": "signature"},
	}
	want := map[string]any{
		"order_id": "order-123", "program_id": "app", "shop_id": "shop", "shopId": "shop",
		"order_status": "paid", "result_code": "CUSTOM_STATUS", "error_msg": "order not found",
		"money": json.Number("12.30"), "enabled": true, "empty": "", "missing": nil,
		"design": "normal", "monkey": "normal", "token_count": json.Number("2"),
		"items": []any{map[string]any{"order_id": "child", "accessToken": "[REDACTED]"}, "ordinary", false, nil},
		"sign":  "[REDACTED]",
	}
	got := redactLogValue(input, "")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected log values: got %#v, want %#v", got, want)
	}
	if input["items"].([]any)[0].(map[string]any)["accessToken"] != "secret" {
		t.Fatal("redaction modified the original payload")
	}
}

func TestLogJSONStringsAndNumberPrecision(t *testing.T) {
	logger := &testLogger{}
	client := &Client{logHook: logger.hook}
	body := []byte(`{"order_id":202611102033483123,"money":12.30,"data":"{\"access_token\":\"nested-secret\",\"money\":12.30,\"order_id\":202611102033483123}","items":"[{\"openid\":\"nested-secret\",\"order_id\":\"child\"}]","description":"unchanged","invalid":"{invalid json}"}`)
	client.logPayload(context.Background(), "response", "/api/query.do", 200, body)
	payload := logger.snapshot()[0].Payload.(map[string]any)
	if payload["order_id"] != json.Number("202611102033483123") || payload["money"] != json.Number("12.30") {
		t.Fatalf("numeric values lost precision: %#v", payload)
	}
	if payload["description"] != "unchanged" || payload["invalid"] != "{invalid json}" {
		t.Fatalf("ordinary strings modified: %#v", payload)
	}
	for _, field := range []string{"data", "items"} {
		value, ok := payload[field].(string)
		if !ok || !json.Valid([]byte(value)) || strings.Contains(value, "nested-secret") || !strings.Contains(value, "[REDACTED]") {
			t.Fatalf("JSON string not sanitized: %s = %v", field, payload[field])
		}
	}
	if !strings.Contains(payload["data"].(string), "202611102033483123") || !strings.Contains(payload["data"].(string), "12.30") {
		t.Fatalf("embedded numeric values lost precision: %v", payload["data"])
	}
}
