package congmingpay

import "context"

type OpenIDByAuthCodeRequest struct {
	AuthCode string `json:"auth_code,omitempty"`
	SubAppID string `json:"sub_appid,omitempty"`
}

func (c *Client) OpenIDByAuthCode(ctx context.Context, r OpenIDByAuthCodeRequest) (*Result, error) {
	return c.call(ctx, "/api/getOpenidByAuthCode.do", r, nil, []string{"auth_code"})
}

type UnionOpenIDRequest struct {
	Code       string `json:"code,omitempty"`
	PaymentApp string `json:"payment_app,omitempty"`
}

func (c *Client) UnionOpenID(ctx context.Context, r UnionOpenIDRequest) (*Result, error) {
	return c.call(ctx, "/api/getUnionOpenid.do", r, nil, []string{"code", "payment_app"})
}
