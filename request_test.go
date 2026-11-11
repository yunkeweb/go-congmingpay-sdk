package congmingpay

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
