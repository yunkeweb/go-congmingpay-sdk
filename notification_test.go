package congmingpay

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"testing"
)

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
