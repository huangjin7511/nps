# 管理接入入口

本页只列接入入口。完整 HTTP 路径见 [管理接口说明](/reference/management-api.md)。

新前端、控制台、SDK 和平台应使用正式 `/api/` 接口；旧页面接口只保留兼容，不建议用于新开发。

## 入口清单

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| `GET` | `/api/system/health` | 健康检查 |
| `GET` | `/api/system/discovery` | 路由、能力、当前 actor 发现 |
| `GET` | `/api/auth/session` | 读取当前 session |
| `POST` | `/api/auth/session` | 创建 cookie session |
| `POST` | `/api/auth/session/logout` | 登出当前 session |
| `POST` | `/api/auth/token` | 签发 standalone token |
| `POST` | `/api/auth/register` | 本地用户注册 |
| `POST` | `/api/access/ip-limit/actions/register` | 登记 `ip_limit` 白名单 |

所有正式 `/api/*` 响应都会返回 `Cache-Control: no-store`。

## 推荐顺序

1. `GET /api/system/health`
2. `GET /api/system/discovery`
3. 无状态调用用 `POST /api/auth/token`
4. 需要 cookie session 时用 session 接口
5. 后续按 discovery 的 `data.routes` 和 `data.actions[]` 调用正式接口

如果配置了 `web_base_url`，实际路径会带同一前缀。客户端应优先使用 discovery 发布的路径。

## Discovery 重点字段

| 字段 | 说明 |
| --- | --- |
| `data.session` | 当前登录态 |
| `data.auth` | 登录延迟、TOTP、PoW、`client_vkey` 登录能力 |
| `data.actor` | 当前 actor、权限和作用域 |
| `data.actions[]` | 当前 actor 可用动作 |
| `data.routes` | 当前 actor 可用路径 |
| `data.extensions.authorization` | 角色和权限目录 |
| `data.extensions.cluster` | 节点能力、协议和平台状态 |

`routes` 和 `actions` 会按 actor 权限、作用域和运行时能力裁剪。匿名 discovery 只发布公开入口。

## Token 登录

`POST /api/auth/token` 适合脚本、前后端分离页面和外部平台。请求体支持 `username` + `password`、可选 `totp`，或单独提交 `verify_key`。

成功后用：

```http
Authorization: Bearer <token>
```

规则：

- 该接口只签发 token，不写 session cookie。
- standalone token 不支持 `X-Node-Token`。
- `client_vkey` 登录得到 `client` principal，只保留单客户端作用域。
- 后续 bearer 请求会按当前本地仓库状态刷新 principal。

## Session 登录

只有明确需要 cookie session 时使用 session 接口。

`POST /api/auth/session` 请求体支持 `username` + `password`、可选 `totp`，或单独提交 `verify_key`。启用验证码时提交 `captcha_id` + `captcha_answer`。收到 `pow_required` 后补交 `pow_nonce` + `pow_bits`。

验证码失败后应重新获取验证码。`pow_bits` 使用 discovery 发布的 `data.security.pow_bits`。

## 注册与 IP 白名单

`POST /api/auth/register` 只在允许注册时可用，请求体使用 `username`、`password`，启用验证码时还要提交 `captcha_id` 和 `captcha_answer`。

`POST /api/access/ip-limit/actions/register` 只登记访问白名单，不创建 session，不签发 token。请求体只需要 `verify_key`。
