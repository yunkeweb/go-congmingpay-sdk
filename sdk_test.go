package congmingpay

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestPHPCompatibleSignature(t *testing.T) {
	params := Params{"null": nil, "empty": "", "bool": false, "money": "12.30", "obj": json.RawMessage(`{"z":"中国/<>","a":1}`)}
	got, err := Sign(params, "test-secret")
	if err != nil {
		t.Fatal(err)
	}
	if got != "9CF2C8AF99BBA53E87C263C337A6EDB1" {
		t.Fatalf("PHP signature mismatch: %s", got)
	}
}

func TestVerifyCallbackFieldOrder(t *testing.T) {
	c, err := NewClient(Config{BaseURL: "https://example.com", ProgramID: "app", ShopID: "shop", Key: "key"})
	if err != nil {
		t.Fatal(err)
	}
	sum := md5.Sum([]byte("z=1&a=2&key=key"))
	n := Notification{"z": "1", "a": "2", "sign": fmt.Sprintf("%X", sum)}
	if _, err := c.VerifyCallback(n, []string{"z", "a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.VerifyCallback(n, []string{"a", "z"}); err == nil {
		t.Fatal("accepted incorrect field order")
	}
	if _, err := c.VerifyCallback(n, nil); err == nil {
		t.Fatal("accepted no signed fields")
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

func TestRequestValidation(t *testing.T) {
	c, err := NewClient(Config{BaseURL: "https://example.com", ProgramID: "app", ShopID: "shop", Key: "key"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.QueryOrder(context.Background(), QueryOrderRequest{}); err == nil {
		t.Error("missing order accepted")
	}
	if _, err := c.Refund(context.Background(), RefundRequest{}); err == nil {
		t.Error("missing refund order accepted")
	}
	if _, err := c.ProfitOrder(context.Background(), ProfitOrderRequest{OrderID: "o", OutTradeNo: "p", IsProfit: "1"}); err == nil {
		t.Error("missing profit rule accepted")
	}
	if _, err := c.SetMerchantWXApp(context.Background(), SetMerchantWXAppRequest{ConfigType: "appid"}); err == nil {
		t.Error("missing appid accepted")
	}
}

func TestJSONCallbackAlternateOrderID(t *testing.T) {
	c, err := NewClient(Config{BaseURL: "https://example.com", ProgramID: "app", ShopID: "shop", Key: "key"})
	if err != nil {
		t.Fatal(err)
	}
	fields := Params{"money": "1.00", "order_id": "order", "result_code": "SUCCESS", "shopId": "shop"}
	sign, _ := Sign(fields, "key")
	fields["sign"] = strings.ToLower(sign)
	body, _ := json.Marshal(fields)
	callback, err := c.ParsePaymentNotificationJSON(body)
	if err != nil || callback.OrderID != "order" || callback.ShopID != "shop" {
		t.Fatalf("bad callback: %v", err)
	}
	fields["orderId"] = "unsigned"
	body, _ = json.Marshal(fields)
	if _, err := c.ParsePaymentNotificationJSON(body); err == nil {
		t.Fatal("accepted unsigned orderId")
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

func TestTypedEndpoints(t *testing.T) {
	expectedPath := ""
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != expectedPath {
			t.Errorf("got path %s, want %s", r.URL.Path, expectedPath)
		}
		_, _ = w.Write([]byte(`{"result_code":"success"}`))
	}))
	defer srv.Close()
	c, err := NewClient(Config{BaseURL: srv.URL, ProgramID: "app", ShopID: "shop", Key: "key", HTTPClient: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	c = c.WithLogHook(nil)
	ctx := context.Background()
	common := PaymentFields{Money: "1.00", OrderID: "order", OrderType: "weixin", Device: "device", NotifyURL: "https://example.com"}
	cases := []struct {
		path string
		call func() (*Result, error)
	}{
		{"/api/buyPay.do", func() (*Result, error) { return c.BuyPay(ctx, BuyPayRequest{PaymentFields: common}) }},
		{"/api/jsNativePay.do", func() (*Result, error) {
			return c.JSNativePay(ctx, JSNativePayRequest{PaymentFields: common, OpenID: "open"})
		}},
		{"/api/microPay.do", func() (*Result, error) {
			return c.MicroPay(ctx, MicroPayRequest{PaymentFields: common, AuthCode: "auth"})
		}},
		{"/api/v3/vprePay.do", func() (*Result, error) {
			return c.PrePay(ctx, PrePayRequest{Money: "1.00", OrderID: "order", NotifyURL: "https://example.com"})
		}},
		{"/api/miniAppPay.do", func() (*Result, error) {
			return c.MiniAppPay(ctx, MiniAppPayRequest{PaymentFields: common, AppID: "app", OpenID: "open"})
		}},
		{"/api/query.do", func() (*Result, error) { return c.QueryOrder(ctx, QueryOrderRequest{OrderID: "order"}) }},
		{"/api/refund.do", func() (*Result, error) { return c.Refund(ctx, RefundRequest{ShopOrderID: "order"}) }},
		{"/api/queryRefundOrder.do", func() (*Result, error) { return c.QueryRefund(ctx, QueryRefundRequest{RefundOrderID: "refund"}) }},
		{"/api/cancelOrder.do", func() (*Result, error) { return c.CancelOrder(ctx, CancelOrderRequest{OrderID: "order"}) }},
		{"/api/profitorder.do", func() (*Result, error) {
			return c.ProfitOrder(ctx, ProfitOrderRequest{OrderID: "order", OutTradeNo: "trade", IsProfit: "1", ProfitRule: json.RawMessage(`[{"shop_id":"recipient","money":1}]`)})
		}},
		{"/api/profitorderback.do", func() (*Result, error) {
			return c.ProfitOrderBack(ctx, ProfitOrderBackRequest{OrderID: "order", ProfitOrderID: "profit", PSShopID: "recipient", PSOrderBackMoney: "1.00", ProfitBackNotifyURL: "https://example.com"})
		}},
		{"/api/searchMerchantWxAppMsg.do", func() (*Result, error) { return c.SearchMerchantWXApp(ctx, SearchMerchantWXAppRequest{}) }},
		{"/api/setMerchantWxAppMsg.do", func() (*Result, error) {
			return c.SetMerchantWXApp(ctx, SetMerchantWXAppRequest{ConfigType: "appid", AppID: "app"})
		}},
		{"/api/getOpenidByAuthCode.do", func() (*Result, error) { return c.OpenIDByAuthCode(ctx, OpenIDByAuthCodeRequest{AuthCode: "auth"}) }},
		{"/api/userCancelOrder.do", func() (*Result, error) {
			return c.UserCancelOrder(ctx, UserCancelOrderRequest{OrderID: "order", ErrorMessage: "cancelled"})
		}},
		{"/api/getUnionOpenid.do", func() (*Result, error) {
			return c.UnionOpenID(ctx, UnionOpenIDRequest{Code: "code", PaymentApp: "app"})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			expectedPath = tc.path
			result, err := tc.call()
			if err != nil {
				t.Fatal(err)
			}
			if result.ResultCode != "success" {
				t.Fatal("missing result code")
			}
		})
	}
}
