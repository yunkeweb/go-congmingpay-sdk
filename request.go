package congmingpay

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
)

// Params contains API field names and values exactly as specified by the gateway.
type Params map[string]any

func (c *Client) call(ctx context.Context, path string, request any, defaults Params, required []string, alternatives ...[]string) (*Result, error) {
	b, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("congmingpay: encode parameters: %w", err)
	}
	params := Params{}
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.UseNumber()
	if err := decoder.Decode(&params); err != nil {
		return nil, err
	}
	var rawFields map[string]json.RawMessage
	if err := json.Unmarshal(b, &rawFields); err != nil {
		return nil, err
	}
	for k, v := range rawFields {
		if len(v) > 0 && (v[0] == '{' || v[0] == '[') {
			params[k] = v
		}
	}
	for k, v := range defaults {
		if _, ok := params[k]; !ok {
			params[k] = v
		}
	}
	for _, k := range required {
		if params[k] == nil || params[k] == "" {
			return nil, fmt.Errorf("congmingpay: %s is required", k)
		}
	}
	for _, group := range alternatives {
		found := false
		for _, k := range group {
			if params[k] != nil && params[k] != "" {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("congmingpay: one of %v is required", group)
		}
	}
	if path == "/api/jsNativePay.do" && !(params["order_type"] == "douyin" && params["pay_type"] == "app") && (params["openid"] == nil || params["openid"] == "") {
		return nil, fmt.Errorf("congmingpay: openid is required")
	}
	if path == "/api/profitorder.do" && params["is_profit"] == "1" && params["auth_money"] == nil && params["profit_rule"] == nil {
		return nil, fmt.Errorf("congmingpay: profit_rule is required when auth_money is absent")
	}
	if path == "/api/setMerchantWxAppMsg.do" {
		if params["config_type"] == "appid" && params["appid"] == nil {
			return nil, fmt.Errorf("congmingpay: appid is required")
		}
		if params["config_type"] == "path" && params["auth_list"] == nil {
			return nil, fmt.Errorf("congmingpay: auth_list is required")
		}
	}
	raw, err := c.Do(ctx, path, params)
	if err != nil {
		return nil, err
	}
	return parseResult(raw), nil
}
