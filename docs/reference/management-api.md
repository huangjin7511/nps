# NPS 管理接口说明

本页说明正式对外管理接口的边界。新前端、控制台、SDK 和平台应使用 `/api/` 接口；旧页面 API 只保留兼容，不建议用于新开发。

如果只是部署单台服务端，优先看 [服务端运维](/guide/server/operations.md)。

## 基本约定

- 正式管理协议统一使用 `/api/` 前缀。
- 节点以本地数据为准，外部平台写入后以节点返回结果为最终结果。
- `GET /api/system/discovery` 是新接入方的首个接口。
- discovery 的 `routes`、`actions`、平台状态都会按当前 actor 裁剪。
- 未认证返回 `401 unauthorized`，已认证但无权限返回 `403 forbidden`。
- `/api/*` 响应返回 `Cache-Control: no-store`。
- 如果配置 `web_base_url=/nps`，实际路径会整体变为 `/nps/api/...`。

## 入口

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| `GET` | `/api/system/health` | 健康检查 |
| `GET` | `/api/system/discovery` | 发现路由和能力 |
| `POST` | `/api/auth/token` | 签发 standalone token |
| `GET` | `/api/auth/session` | 读取 session |
| `POST` | `/api/auth/session` | 创建 session |
| `POST` | `/api/auth/session/logout` | 登出 |
| `POST` | `/api/auth/register` | 用户注册 |
| `POST` | `/api/access/ip-limit/actions/register` | 登记 `ip_limit` 白名单 |

## 认证方式

| 方式 | 推荐用途 | 凭证位置 |
| --- | --- | --- |
| session | 本地 Web 页面、同源浏览器 | cookie |
| standalone token | 前后端分离、脚本、服务到服务 | `Authorization: Bearer <token>` |
| platform token | 外部平台、多节点控制面 | `X-Node-Token` 或 `Authorization: Bearer <token>` |

platform token 的 `POST` JSON body 也支持 `token` 或 `node_token`。`GET` 请求不会从 URL query 读取 token。

本地身份：

| 身份 | 说明 |
| --- | --- |
| `admin` | 本地管理员 |
| `user` | 本地普通用户 |
| `client` | `client_vkey` 登录得到的单客户端 principal |

## 平台 actor

平台请求可通过 header 指定委托上下文：

| Header | 说明 |
| --- | --- |
| `X-Platform-Role` | `admin` 或 `user` |
| `X-Platform-Username` | 平台用户名 |
| `X-Platform-Actor-ID` | 平台用户 ID |
| `X-Platform-Client-IDs` | 可见客户端 ID 列表 |

同名字段也支持放入 `GET` query 或 `POST` JSON body，但推荐使用 header，避免泄漏到代理日志。

作用域规则：

- 只有 token、没有委托上下文时，默认按平台管理员处理。
- `full` 平台密钥等同节点管理员。
- `account` 平台管理员可管理本平台 service user 作用域资源。
- 带委托上下文但没有显式角色时，默认按用户作用域处理。
- 本地普通用户只能访问自己 owner 或 manager 的资源镜像，以及自己的统计快照。
- `client` principal 只能访问单客户端作用域，可管理该客户端下的隧道和域名，不能修改客户端自身配置。

敏感字段会按 actor 权限脱敏，例如 `verify_key`、`password`、`auth`、`cert_file`、`key_file`。

## 同步流程

外部平台首次接入推荐顺序：

1. `GET /api/system/discovery`
2. `GET /api/system/overview`
3. 有管理员权限时可用 `GET /api/system/overview?config=1`
4. 建立 `GET /api/ws`
5. 用 `GET /api/system/changes` 补偿断线期间变更

遇到以下情况应全量同步：

- `boot_id` 变化
- `config_epoch` 变化
- `/api/system/changes` 返回 `gap=true`
- WS 收到 `epoch_changed` 或 `resync_required`

完整管理员备份使用 `GET /api/system/export`，恢复使用 `POST /api/system/import`。

## 文档索引

| 任务 | 页面 |
| --- | --- |
| 查接入入口和登录 | [管理接入入口](/reference/integration/management-api-entrypoints.md) |
| 查 HTTP 路径 | [HTTP 接口目录](/reference/management-api-http.md) |
| 查实时事件、WS、callback、webhook | [实时通道与回调](/reference/management-api-realtime.md) |
