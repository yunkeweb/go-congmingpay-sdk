package congmingpay

import (
	"bytes"
	"crypto/md5" // Required by the provider's protocol.
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
)

// Notification contains decoded callback fields. Only the fields specified in
// the relevant verification function are authenticated by the provider's MD5.
// Confirm merchant, order, amount and status against your own records and query
// the gateway before taking action; all other fields are untrusted.
type Notification map[string]string

// Callback contains only authenticated fields as typed values. Raw includes
// all callback fields; fields other than those listed below are unauthenticated.
type Callback struct {
	Money      string
	OrderID    string
	ResultCode string
	ShopID     string
	Raw        Notification
}

// ParsePaymentNotification verifies a payment form callback and returns its signed fields.
func (c *Client) ParsePaymentNotification(values url.Values) (*Callback, error) {
	n, err := c.VerifyPaymentNotification(values)
	return paymentCallback(n, err)
}

// ParsePaymentNotificationJSON verifies a JSON payment callback. Confirm the
// provider's new callback signing scheme before relying on it in production.
func (c *Client) ParsePaymentNotificationJSON(body []byte) (*Callback, error) {
	n, err := c.VerifyPaymentNotificationJSON(body)
	return paymentCallback(n, err)
}

func paymentCallback(n Notification, err error) (*Callback, error) {
	if err != nil {
		return nil, err
	}
	orderID, ok := n["orderId"]
	if !ok {
		orderID = n["order_id"]
	}
	return &Callback{Money: n["money"], OrderID: orderID, ResultCode: n["result_code"], ShopID: n["shopId"], Raw: n}, nil
}

// ParseRefundNotification verifies a refund form callback and returns its signed fields.
func (c *Client) ParseRefundNotification(values url.Values) (*Callback, error) {
	n, err := c.VerifyRefundNotification(values)
	if err != nil {
		return nil, err
	}
	return &Callback{Money: n["money"], OrderID: n["order_id"], ResultCode: n["result_code"], ShopID: n["shop_id"], Raw: n}, nil
}

// VerifyPaymentNotification verifies a GET/form payment callback using the PHP
// SDK's field selection. An order_id is never substituted when orderId exists.
func (c *Client) VerifyPaymentNotification(values url.Values) (Notification, error) {
	n, err := notificationFromValues(values)
	if err != nil {
		return nil, err
	}
	return c.verifyPaymentNotification(n)
}

// VerifyRefundNotification verifies a GET/form refund callback. The provider
// signs only money, order_id, result_code and shop_id.
func (c *Client) VerifyRefundNotification(values url.Values) (Notification, error) {
	n, err := notificationFromValues(values)
	if err != nil {
		return nil, err
	}
	return c.verifyNotification(n, []string{"money", "order_id", "result_code", "shop_id"})
}

// VerifyPaymentNotificationJSON verifies a JSON payment callback using the
// four-field payment signature shown in section 2-23 of the provider document.
// The document does not explicitly restate the new callback signature scheme;
// confirm it with the provider and test before production use.
func (c *Client) VerifyPaymentNotificationJSON(body []byte) (Notification, error) {
	n, err := notificationFromJSON(body)
	if err != nil {
		return nil, err
	}
	return c.verifyPaymentNotification(n)
}

func (c *Client) verifyPaymentNotification(n Notification) (Notification, error) {
	orderField := "orderId"
	if _, ok := n[orderField]; !ok {
		orderField = "order_id"
	}
	return c.verifyNotification(n, []string{"money", orderField, "result_code", "shopId"})
}

// VerifyCallback verifies explicitly ordered callback fields. Callers must know
// the provider's signing rule for the specific callback; unsigned fields remain
// untrusted. Missing and empty signed fields are rejected.
func (c *Client) VerifyCallback(n Notification, names []string) (Notification, error) {
	return c.verifyNotification(n, names)
}

func (c *Client) verifyNotification(n Notification, names []string) (Notification, error) {
	if c == nil {
		return nil, errors.New("congmingpay: nil client")
	}
	if len(names) == 0 {
		return nil, errors.New("congmingpay: signed callback fields are required")
	}
	pairs := make([]string, 0, len(names)+1)
	seen := make(map[string]bool, len(names))
	for _, name := range names {
		if name == "sign" || seen[name] {
			return nil, errors.New("congmingpay: invalid signed callback fields")
		}
		seen[name] = true
		value, ok := n[name]
		if !ok || value == "" {
			return nil, fmt.Errorf("congmingpay: missing signed callback field %s", name)
		}
		pairs = append(pairs, name+"="+value)
	}
	actual, ok := n["sign"]
	if !ok || actual == "" {
		return nil, errors.New("congmingpay: missing callback signature")
	}
	pairs = append(pairs, "key="+c.key)
	sum := md5.Sum([]byte(strings.Join(pairs, "&")))
	expected := fmt.Sprintf("%X", sum)
	if !equalSignature(expected, actual) {
		return nil, errors.New("congmingpay: invalid callback signature")
	}
	return n, nil
}

func notificationFromValues(values url.Values) (Notification, error) {
	n := make(Notification, len(values))
	for k, v := range values {
		if len(v) != 1 {
			return nil, fmt.Errorf("congmingpay: duplicate or missing callback field %s", k)
		}
		n[k] = v[0]
	}
	return n, nil
}

func notificationFromJSON(body []byte) (Notification, error) {
	if len(body) > 1<<20 {
		return nil, errors.New("congmingpay: callback exceeds 1 MiB")
	}
	d := json.NewDecoder(bytes.NewReader(body))
	d.UseNumber()
	tok, err := d.Token()
	if err != nil || tok != json.Delim('{') {
		return nil, errors.New("congmingpay: callback must be a JSON object")
	}
	n := Notification{}
	for d.More() {
		tok, err := d.Token()
		if err != nil {
			return nil, errors.New("congmingpay: malformed callback")
		}
		key, ok := tok.(string)
		if !ok {
			return nil, errors.New("congmingpay: malformed callback key")
		}
		if _, exists := n[key]; exists {
			return nil, fmt.Errorf("congmingpay: duplicate callback field %s", key)
		}
		var raw json.RawMessage
		if err := d.Decode(&raw); err != nil {
			return nil, errors.New("congmingpay: malformed callback value")
		}
		var value string
		if len(raw) > 0 && raw[0] == '"' {
			if err := json.Unmarshal(raw, &value); err != nil {
				return nil, errors.New("congmingpay: malformed callback string")
			}
		} else {
			value = string(raw)
		}
		n[key] = value
	}
	if tok, err = d.Token(); err != nil || tok != json.Delim('}') {
		return nil, errors.New("congmingpay: malformed callback")
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, errors.New("congmingpay: trailing callback data")
	}
	return n, nil
}
