package congmingpay

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
)

// Notification contains decoded callback fields. Only the fields specified in
// the relevant verification function are authenticated by the provider's MD5.
// Confirm merchant, order, amount and status against your own records and query
// the gateway before taking action; all other fields are untrusted.
type Notification map[string]string

// VerifyPaymentNotification verifies a legacy GET/form payment callback.
// The provider signs money, orderId, result_code and shopId only. In particular,
// do not use an unsigned order_id in place of the signed orderId.
func (c *Client) VerifyPaymentNotification(values url.Values) (Notification, error) {
	n, err := notificationFromValues(values)
	if err != nil {
		return nil, err
	}
	return c.verifyNotification(n, []string{"money", "orderId", "result_code", "shopId"})
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
	return c.verifyNotification(n, []string{"money", "orderId", "result_code", "shopId"})
}

func (c *Client) verifyNotification(n Notification, names []string) (Notification, error) {
	if c == nil {
		return nil, errors.New("congmingpay: nil client")
	}
	fields := make(Params, len(names))
	for _, name := range names {
		value, ok := n[name]
		if !ok || value == "" {
			return nil, fmt.Errorf("congmingpay: missing signed callback field %s", name)
		}
		fields[name] = value
	}
	actual, ok := n["sign"]
	if !ok || actual == "" {
		return nil, errors.New("congmingpay: missing callback signature")
	}
	expected, err := Sign(fields, c.key)
	if err != nil {
		return nil, err
	}
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
