package congmingpay

import "context"

type SearchMerchantWXAppRequest struct{}

func (c *Client) SearchMerchantWXApp(ctx context.Context, r SearchMerchantWXAppRequest) (*Result, error) {
	return c.call(ctx, "/api/searchMerchantWxAppMsg.do", r, nil, nil)
}

type SetMerchantWXAppRequest struct {
	ConfigType string `json:"config_type,omitempty"`
	AppID      string `json:"appid,omitempty"`
	AuthList   string `json:"auth_list,omitempty"`
}

func (c *Client) SetMerchantWXApp(ctx context.Context, r SetMerchantWXAppRequest) (*Result, error) {
	return c.call(ctx, "/api/setMerchantWxAppMsg.do", r, nil, []string{"config_type"})
}
