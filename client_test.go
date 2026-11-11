package congmingpay

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

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

func TestMerchantIsolationAndDefaults(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var data Params
		dec := json.NewDecoder(r.Body)
		dec.UseNumber()
		if err := dec.Decode(&data); err != nil {
			t.Error(err)
			return
		}
		shop, ok := data["shop_id"].(string)
		if !ok {
			t.Error("missing shop_id")
			return
		}
		if data["program_id"] != "app-"+shop {
			t.Error("incorrect program id")
		}
		sign, err := Sign(data, "key-"+shop)
		if err != nil || sign != data["sign"] {
			t.Error("signature mismatch")
		}
		if r.URL.Path == "/api/buyPay.do" && (data["ver"] != "3.0" || data["profit_share_type"] != "0" || data["is_notify_new"] != "0") {
			t.Error("missing defaults")
		}
		if r.URL.Path == "/api/query.do" {
			if _, ok := data["ver"]; ok {
				t.Error("unexpected version")
			}
		}
		_, _ = w.Write([]byte(`{"result_code":"success","order_status":"2"}`))
	}))
	defer srv.Close()
	clients := make([]*Client, 2)
	for i, shop := range []string{"one", "two"} {
		c, err := NewClient(Config{BaseURL: srv.URL, ProgramID: "app-" + shop, ShopID: shop, Key: "key-" + shop, HTTPClient: srv.Client()})
		if err != nil {
			t.Fatal(err)
		}
		clients[i] = c.WithLogHook(nil)
	}
	var wg sync.WaitGroup
	for _, c := range clients {
		wg.Add(1)
		go func(c *Client) {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				r, err := c.QueryOrder(context.Background(), QueryOrderRequest{OrderID: "order"})
				if err != nil || r.OrderStatus != "2" {
					t.Errorf("query failed: %v", err)
					return
				}
			}
		}(c)
	}
	wg.Wait()
	_, err := clients[0].BuyPay(context.Background(), BuyPayRequest{PaymentFields: PaymentFields{Money: "12.30", OrderID: "order", OrderType: "weixin", Device: "device", NotifyURL: "https://example.com/callback"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"shop_id", "program_id", "sign"} {
		if _, err := clients[0].Do(context.Background(), "/api/custom.do", Params{field: "override"}); err == nil {
			t.Errorf("accepted override %s", field)
		}
	}
}

type changingJSON struct{ calls *int }

func (v changingJSON) MarshalJSON() ([]byte, error) {
	*v.calls++
	if *v.calls == 1 {
		return []byte(`{"z":1,"a":2}`), nil
	}
	return []byte(`{"z":9}`), nil
}

func TestRequestSnapshotAndDecimal(t *testing.T) {
	count := 0
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var raw map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			t.Error(err)
			return
		}
		params := Params{}
		for k, v := range raw {
			params[k] = v
		}
		sign, err := Sign(params, "key")
		var actual string
		_ = json.Unmarshal(raw["sign"], &actual)
		if err != nil || sign != actual {
			t.Error("signed data differs from request")
		}
		if r.URL.Path == "/api/custom.do" && string(raw["object"]) != `{"z":1,"a":2}` {
			t.Errorf("changed object: %s", raw["object"])
		}
		if r.URL.Path == "/api/profitorderback.do" && string(raw["ps_order_back_money"]) != "12.30" {
			t.Errorf("decimal changed: %s", raw["ps_order_back_money"])
		}
		_, _ = w.Write([]byte(`{"result_code":"success"}`))
	}))
	defer srv.Close()
	c, err := NewClient(Config{BaseURL: srv.URL, ProgramID: "app", ShopID: "shop", Key: "key", HTTPClient: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	c = c.WithLogHook(nil)
	if _, err := c.Do(context.Background(), "/api/custom.do", Params{"object": changingJSON{calls: &count}}); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("custom marshaler called %d times", count)
	}
	_, err = c.ProfitOrderBack(context.Background(), ProfitOrderBackRequest{OrderID: "o", ProfitOrderID: "p", PSShopID: "recipient", PSOrderBackMoney: "12.30", ProfitBackNotifyURL: "https://example.com"})
	if err != nil {
		t.Fatal(err)
	}
}
