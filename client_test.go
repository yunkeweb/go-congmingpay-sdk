package congmingpay

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestSign(t *testing.T) {
	sum := md5.Sum([]byte("a=one&b=two&key=secret"))
	got, err := Sign(Params{"b": "two", "a": "one", "sign": "ignored"}, "secret")
	if err != nil || got != fmt.Sprintf("%X", sum) {
		t.Fatalf("signature %s: %v", got, err)
	}
}

func TestRequestAndResponse(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/api/query.do" || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var p Params
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			t.Error(err)
			return
		}
		sign, _ := Sign(p, "secret")
		if p["program_id"] != "app" || p["sign"] != sign {
			t.Errorf("invalid signature or program ID")
		}
		_, _ = w.Write([]byte(`{"result_code":"success","order_status":"2"}`))
	}))
	defer srv.Close()
	c, err := NewClient(Config{BaseURL: srv.URL, ProgramID: "app", ShopID: "shop", Key: "secret", HTTPClient: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	params := QueryOrderRequest{OrderID: "order"}
	r, err := c.QueryOrder(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	if r.ResultCode != "success" || r.OrderStatus != "2" || params.OrderID != "order" {
		t.Fatalf("unexpected response or mutation")
	}
	if _, err = c.Do(context.Background(), "/api/query.do", Params{"sign": "bad"}); err == nil {
		t.Fatal("accepted caller signature")
	}
	if _, err = c.Do(context.Background(), "//example.com/api/query.do", nil); err == nil {
		t.Fatal("accepted external path")
	}
}

func TestRedirectIsNotFollowed(t *testing.T) {
	redirected := false
	target := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { redirected = true }))
	defer target.Close()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer srv.Close()
	c, err := NewClient(Config{BaseURL: srv.URL, ProgramID: "app", ShopID: "shop", Key: "secret", HTTPClient: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.QueryOrder(context.Background(), QueryOrderRequest{OrderID: "order"})
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) || httpErr.StatusCode != 307 || redirected {
		t.Fatalf("unexpected redirect: %v", err)
	}
}

func TestNotifications(t *testing.T) {
	c, err := NewClient(Config{BaseURL: "https://example.com", ProgramID: "app", ShopID: "shop", Key: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	fields := Params{"money": "1.00", "orderId": "order", "result_code": "SUCCESS", "shopId": "shop"}
	sign, err := Sign(fields, "secret")
	if err != nil {
		t.Fatal(err)
	}
	v := url.Values{"money": {"1.00"}, "orderId": {"order"}, "result_code": {"SUCCESS"}, "shopId": {"shop"}, "sign": {sign}, "order_id": {"unsigned"}}
	if _, err := c.VerifyPaymentNotification(v); err != nil {
		t.Fatal(err)
	}
	v["money"] = []string{"2.00"}
	if _, err := c.VerifyPaymentNotification(v); err == nil {
		t.Fatal("accepted modified amount")
	}
	v["money"] = []string{"1.00", "2.00"}
	if _, err := c.VerifyPaymentNotification(v); err == nil {
		t.Fatal("accepted duplicate amount")
	}
	body, _ := json.Marshal(Params{"money": "1.00", "orderId": "order", "result_code": "SUCCESS", "shopId": "shop", "sign": sign})
	if _, err := c.VerifyPaymentNotificationJSON(body); err != nil {
		t.Fatal(err)
	}
	if _, err := c.VerifyPaymentNotificationJSON([]byte(`{"money":"1.00","money":"1.00"}`)); err == nil {
		t.Fatal("accepted duplicate JSON key")
	}
	refund := Params{"money": "-1.00", "order_id": "refund", "result_code": "SUCCESS", "shop_id": "shop"}
	rs, _ := Sign(refund, "secret")
	rv := url.Values{"money": {"-1.00"}, "order_id": {"refund"}, "result_code": {"SUCCESS"}, "shop_id": {"shop"}, "sign": {rs}}
	if _, err := c.VerifyRefundNotification(rv); err != nil {
		t.Fatal(err)
	}
}
