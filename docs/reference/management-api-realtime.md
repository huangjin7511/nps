# 管理接口：实时通道与回调

本页列 WS、事件订阅、callback 和 webhook。HTTP 资源接口见 [HTTP 接口目录](/reference/management-api-http.md)。

## WebSocket

| 项 | 说明 |
| --- | --- |
| 入口 | `GET /api/ws` |
| 请求路径 | `request.path` 复用正式 `/api/` 管理路径 |
| 方法 | `GET`、`POST` |
| 浏览器同源 | 使用当前 session |
| 浏览器 token | standalone token 放入 `Sec-WebSocket-Protocol` |
| 非浏览器 token | `Authorization: Bearer <token>` 或 `X-Node-Token` |

如果配置了 `web_base_url`，WS 入口和 `request.path` 都要带同一前缀。

常用帧：`hello`、`request`、`response`、`event`、`callback`、`ping`、`pong`、`epoch_changed`、`resync_required`、`error`。

最小请求：

```json
{
  "type": "request",
  "id": "req-1",
  "method": "GET",
  "path": "/api/system/status"
}
```

规则：

- 成功和业务错误优先放在 `response.body`，结构与 HTTP 接口一致。
- `response.error` 主要表示协议级失败。
- `/api/batch` 可通过 WS 调用，但不能嵌套 `/api/batch`，也不能转发 WS-only 路径。
- `/api/ws` 不能作为子请求调用。
- 写请求可带 `X-Operation-ID`。
- 收到 `epoch_changed` 或 `resync_required` 后应全量同步。

## WS 临时订阅

这组路径只在 WS `request.path` 内可用，不是 HTTP 接口。

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| `GET` | `/api/realtime/subscriptions` | 列表 |
| `POST` | `/api/realtime/subscriptions` | 创建 |
| `GET` | `/api/realtime/subscriptions/{id}` | 详情 |
| `POST` | `/api/realtime/subscriptions/{id}/actions/update` | 更新 |
| `POST` | `/api/realtime/subscriptions/{id}/actions/status` | 启停 |
| `POST` | `/api/realtime/subscriptions/{id}/actions/delete` | 删除 |

订阅命中后，原始 `event` 仍会发送，并额外发送 `callback` 帧。连接断开后订阅自动释放。

选择器字段：`event_names`、`resources`、`actions`、`user_ids`、`client_ids`、`tunnel_ids`、`host_ids`。

内容字段：`content_mode`、`content_type`、`body_template`、`header_templates`。`canonical` 返回标准事件 envelope，`custom` 使用模板渲染。

## Reverse WS

当平台配置为 `reverse` 或 `dual` 时，节点会主动连接平台的 `reverse_ws_url`。

流程：

1. 节点发送 `hello`。
2. 平台返回 `hello`，可带 `last_boot_id`、`changes_after`、`changes_limit`。
3. 节点返回 `hello_ack`。
4. 后续帧模型与普通 `/api/ws` 相同。

如果 `boot_id` 变化、`config_epoch` 变化或回放出现 `gap`，平台应重新全量同步。

## Callback 队列

配置驱动的 `platform_callback_*` 会把事件投递到平台 `callback_url`。失败后进入本地队列。

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| `GET` | `/api/callbacks/queue` | 查看失败队列 |
| `POST` | `/api/callbacks/queue/actions/replay` | 重放失败队列 |
| `POST` | `/api/callbacks/queue/actions/clear` | 清空失败队列 |

`GET` 支持 `platform_id`、`limit`。replay / clear 的 JSON body 支持 `platform_id`。这些接口只在当前 actor 可见 callback-enabled 平台时发布。

常见投递头：`Authorization`、`X-Node-Token`、`X-Node-ID`、`X-Platform-ID`、`X-Node-Schema-Version`、`X-Node-Boot-ID`、`X-Request-ID`。配置签名 key 后会发送 `X-Node-Signature-*`。

## Webhook

Webhook 是运行时可管理的持久化事件 sink，独立于 `platform_callback_*`。

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| `GET` | `/api/webhooks` | 列表 |
| `POST` | `/api/webhooks` | 创建 |
| `GET` | `/api/webhooks/{id}` | 详情 |
| `POST` | `/api/webhooks/{id}/actions/update` | 更新 |
| `POST` | `/api/webhooks/{id}/actions/status` | 启停 |
| `POST` | `/api/webhooks/{id}/actions/delete` | 删除 |

Webhook 只开放给本地管理员和 `platform_admin`。普通用户、客户端和 delegated `platform_user` 如需事件推送，应使用 WS 临时订阅。

常用字段：`name`、`url`、`enabled`、`timeout_seconds`，以及与 WS 订阅相同的选择器和内容字段。

## Live-only 事件

这些事件会通过 WS 推送，但不进入 durable changes，也不会再次投递给 callback 或 webhook：`operations.updated`、`management_platforms.updated`、`callbacks_queue.updated`、`webhook.delivery_succeeded`、`webhook.delivery_failed`。
