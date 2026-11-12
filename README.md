# go-congmingpay-sdk

聪明付基础支付 API 的 Go SDK，基于飞书文档 revision 3121 和 PHP SDK 的实现。仅依赖 Go 标准库，需要 Go 1.22 或更高版本。

## 安装

```sh
go get github.com/yunkeweb/go-congmingpay-sdk
```

## 客户端

每个 Client 对应一组商户配置。可以同时创建多个 Client；不要把真实密钥提交到代码仓库。

```go
client, err := congmingpay.NewClient(congmingpay.Config{
    BaseURL:   os.Getenv("CONGMINGPAY_BASE_URL"),
    ProgramID: os.Getenv("CONGMINGPAY_PROGRAM_ID"),
    ShopID:    os.Getenv("CONGMINGPAY_SHOP_ID"),
    Key:       os.Getenv("CONGMINGPAY_KEY"),
})
if err != nil {
    return err
}

result, err := client.QueryOrder(ctx, congmingpay.QueryOrderRequest{
    OrderID: "merchant-order-id",
})
if err != nil {
    return err
}
log.Printf("result=%s status=%s", result.ResultCode, result.OrderStatus)
```

可通过 `Config.HTTPClient` 提供自定义 HTTP 客户端；默认超时 30 秒。SDK 要求 HTTPS，不跟随重定向，不自动重试。`program_id`、`shop_id` 和 `sign` 由客户端注入，不能在通用请求中覆盖。分账接收方字段不受此限制。Client 可并发使用，HTTP 客户端和日志钩子也必须支持并发。

接口方法包括 `BuyPay`、`JSNativePay`、`MicroPay`、`PrePay`、`MiniAppPay`、`QueryOrder`、`Refund`、`QueryRefund`、`CancelOrder`、`ProfitOrder`、`ProfitOrderBack`、`SearchMerchantWXApp`、`SetMerchantWXApp`、`OpenIDByAuthCode`、`UserCancelOrder` 和 `UnionOpenID`。方法的请求结构体以方法名加 `Request` 命名。

`BuyPayRequest`、`JSNativePayRequest`、`MicroPayRequest` 和 `MiniAppPayRequest` 通过嵌入的 `PaymentFields` 设置共同字段。金额通常使用字符串，单位和精度遵循对应接口；分账回退的 JSON 数值使用 `json.Number` 保留原始十进制表示。指定版本的接口自动填充 `3.0`；查询接口的可选 `Ver` 仅在显式指定时发送。支付接口的分账及回调格式默认值与 PHP SDK 一致，可通过请求字段覆盖。

每个类型化方法返回 `Result`。常用字段可直接读取，`Result.Raw` 保留完整 JSON 响应字段；使用 `json.Unmarshal(result.Raw["字段名"], &value)` 可解析其他字段。`error == nil` 只表示 HTTP 和 JSON 处理成功，不能说明交易成功；必须检查业务状态。请求超时不代表交易未发生，重试支付、退款前应先查询，避免重复操作。

订单标识的参数名随接口而异：下单和 `QueryOrderRequest.OrderID` 的 `order_id` 传商户订单号，退款按商户订单号定位时使用 `RefundRequest.ShopOrderID` 的 `shop_order_id`。不要仅因字段都叫 `OrderID` 就跨接口互换。`RefundOrderID` 是调用方传入的退款单号，用于后续 `QueryRefundRequest.RefundOrderID`；平台退款单号对应 `PlatformRefundOrderID` / `plat_refund_order_id`。这些字段映射不代表平台承诺了幂等，其他订单号和通道流水号仍按对应接口文档传入。

未封装接口可用 `Do(ctx, "/api/custom.do", congmingpay.Params{...})`，返回原始 `Response`。路径必须是 `/api/` 下的相对路径。参数中顶层 `nil` 不发送；涉及嵌套对象的签名需要与平台联调确认。若字段在文档中是 JSON 字符串，应传入字符串，而不是数组或对象。

类型化入口检查已封装的必填字段、至少一项及部分依赖条件；`Do` 只检查通用路径、商户字段和序列化约束，不执行各接口的专属校验。SDK 不自行限制未确认的金额格式、枚举或回调 URL，不修改订单号或金额。

## 读取响应和诊断错误

`Response.String` 和 `Result` 的快捷字符串字段保持原有行为：缺失、JSON null 或非字符串时返回空字符串。需要明确区分时使用 `StringField`：

```go
device, present, err := result.Raw.StringField("device_name")
if err != nil {
    // 可通过 errors.As(err, &fieldTypeErr) 读取 *congmingpay.FieldTypeError。
    return err
}
if present {
    log.Printf("device=%q", device)
}
```

缺失返回 `"", false, nil`，空字符串返回 `"", true, nil`；JSON null、数字、布尔值、数组或对象返回 `*FieldTypeError`。平台字符串 `"null"` 按原值返回。未知业务状态不被解释成成功或失败。

使用 `Raw.Decode` 可一次读取自己需要的字段，无须 SDK 为每个端点维护另一套响应结构。尚未确认类型的字段可以保留为 `json.RawMessage`：

```go
var fields struct {
    OrderKey  json.RawMessage `json:"order_key"`
    AppIDList json.RawMessage `json:"appid_list"`
    AuthList  json.RawMessage `json:"auth_list"`
    RefundMsg json.RawMessage `json:"refund_msg"`
}
if err := result.Raw.Decode(&fields); err != nil {
    return err
}
```

`Decode` 遵循 `json.Unmarshal` 语义：目标必须是非 nil 指针，未知字段被忽略，不检查必填，不自动转换字符串数字。nil `Response` 返回错误；解码失败可能已填入部分目标字段，建议使用新的局部变量。目标含 `any` 时数字默认解析为 `float64`，需要精确保留时使用 `json.Number`、`json.RawMessage` 或自行使用 `Decoder.UseNumber`。`Raw` 不变，但它是字段级 JSON 数据，不保留完整 HTTP 正文的空白、键顺序和重复键。

非 2xx 返回 `*HTTPError`，即使关闭日志也可取得状态码、接口路径、最多 4 MiB 的原始正文、`X-Request-ID` 和原文 `Retry-After`：

```go
var httpErr *congmingpay.HTTPError
if errors.As(err, &httpErr) {
    log.Printf("HTTP status=%d path=%s request_id=%s body_bytes=%d truncated=%t",
        httpErr.StatusCode, httpErr.Path, httpErr.RequestID,
        len(httpErr.Body), httpErr.BodyTruncated)
    // httpErr.Body 是原始正文，由调用方决定如何脱敏、记录或展示。
}
```

`BodyTruncated` 仅表示正文超过大小上限；读取中途失败时保留部分正文，`Cause` 和 `Unwrap()` 保留错误链，支持 `errors.Is(err, context.Canceled)` 等检查。`Error()` 只输出状态码和路径，不自动输出正文、诊断头或底层错误文本。响应头存在时才填入诊断字段；SDK 不因 `Retry-After` 自动重试。HTTP 200 的业务 `fail` 仍返回 `Result` 和 nil error。

## 回调和签名

请求签名参考 PHP SDK：排除 `sign` 和顶层 null，按字段名排序后拼接并使用大写 MD5。对于嵌套对象，签名与发送的 JSON 字节保持一致；跨语言的对象字段顺序、数字格式等仍需通过平台样例核对。

`ParsePaymentNotification` 和 `ParseRefundNotification` 接收 `url.Values`；`ParsePaymentNotificationJSON` 接收 JSON 原文。它们返回的 `Callback` 中 `Money`、`OrderID`、`ResultCode`、`ShopID` 是参与验签的字段，`Raw` 中其他字段并未经过认证。支付回调优先验证 `orderId`，不存在时按 PHP 实现验证 `order_id`；退款回调使用 `shop_id`。实际新回调格式须与平台确认并联调。解析 JSON 前调用方应限制请求体大小。

验签后仍要核对商户、订单、金额和状态，并进行幂等处理及必要的订单查询。响应签名和分账回退回调的签名规则尚不明确，SDK 不声称已验证。退款回调响应格式也须与平台确认。

## 日志

默认使用 `fmt.Printf` 输出请求、响应和错误，Payload 字段默认全部显示。使用者可通过 `Config.LogRedactFields` 指定需要遮蔽的字段名；匹配不区分大小写，并忽略字段名中的下划线和连字符，例如配置 `sub_openid` 会匹配 `subOpenid`。规则同样应用于嵌套对象、JSON 字符串以及其中完整 HTTP(S) URL 的查询参数。非 JSON 内容原样记录。`client.WithLogHook(hook)` 返回一个使用自定义钩子的客户端副本；传入 `nil` 关闭日志。钩子同步调用，必须支持并发且不应阻塞。日志不包含回调验签。

例如在创建客户端时设置 `LogRedactFields: []string{"sign", "sub_openid"}`，仅匹配这两个字段名及上述写法变体，将其值替换为 `[REDACTED]`；`unionid` 等未配置字段继续显示。配置 `token` 不会匹配 `access_token` 或 `token_count`，需要分别列出。配置 `pro_access_token` 时，JSON 中 `https://pay.example/?pro_access_token=secret&order_id=123` 的日志值会变成 `https://pay.example/?pro_access_token=%5BREDACTED%5D&order_id=123`；实际返回的 URL 不变。URL 参数名经过 URL 解码后匹配，同名重复参数都会处理，未匹配参数的值和编码保留。

未设置或设置为空列表时不脱敏。SDK 不内置敏感字段名单，不默认遮蔽整个 URL，也不扫描自由文本、HTML、URL 路径、userinfo 或 fragment。无法解析为完整 HTTP(S) URL 的字符串原样保留。需要处理这些内容时，在自定义 Hook 中决定如何记录；也可将 `url` 等整个 JSON 字段加入配置。

## 项目结构

SDK 保持单一 `congmingpay` 包，按职责拆分文件。业务文件中将请求结构体与对应接口方法放在一起，使用者继续通过模块根路径导入。

| 文件 | 职责 |
| --- | --- |
| `client.go` | 商户配置、客户端创建、HTTP 请求与错误处理 |
| `request.go` | 通用参数、请求转换、默认值与参数校验 |
| `response.go` | 原始响应、通用结果与字段解析 |
| `payment.go` | 各类支付接口及其请求结构体 |
| `order.go` | 订单查询、撤销与取消 |
| `refund.go` | 退款与退款查询 |
| `profit.go` | 分账与分账回退 |
| `merchant.go` | 商户微信应用配置与查询 |
| `openid.go` | OpenID 获取 |
| `notification.go`、`sign.go` | 回调解析、验签与请求签名 |
| `logging.go` | 日志钩子与可配置脱敏 |

测试按客户端、HTTP 错误、请求、响应、签名、回调与日志职责放在对应的 `*_test.go` 文件中；GitHub Actions 工作流放在 `.github/workflows/`。

## 测试与发布

```sh
go test ./...
go test -race ./...
go vet ./...
```

GitHub Actions 执行格式、测试和构建检查，并支持通过版本标签创建 Release。已发布的标签不应修改。当前模块路径支持 v0 和 v1；发布 v2 需要调整模块路径。各版本的兼容性变化见 [更新日志](CHANGELOG.md)。

本地 HTTP/TLS 模拟测试用于验证 SDK 协议行为，不代表全部端点已完成真实支付、退款、分账及回调闭环。
