# 管理接口：发现与快照

本页列读取型系统接口。所有正式 `/api/*` 响应都使用 `data/meta` 结构，并返回 `Cache-Control: no-store`。

## 接口

| 方法 | 路径 | 用途 | 鉴权 |
| --- | --- | --- | --- |
| `GET` | `/api/system/health` | 健康检查 | 无需鉴权 |
| `GET` | `/api/system/discovery` | 路由、能力和 actor 发现 | 可匿名 |
| `GET` | `/api/system/overview` | 推荐总览快照 | 需鉴权 |
| `GET` | `/api/system/dashboard` | 仪表盘统计 | 管理员或 `full` 平台 |
| `GET` | `/api/system/registration` | 节点注册信息 | 需节点状态权限 |
| `GET` | `/api/system/status` | 节点状态和运行参数 | 需节点状态权限 |
| `GET` | `/api/system/operations` | 操作摘要 | 需节点状态权限 |
| `GET` | `/api/system/changes` | 增量事件补偿 | 需鉴权 |
| `GET` | `/api/system/usage-snapshot` | 当前作用域统计 | 需鉴权 |
| `GET` | `/api/system/export` | 完整业务配置导出 | 管理员或 `full` 平台 |

## 推荐同步顺序

1. `GET /api/system/health`
2. `GET /api/system/discovery`
3. `GET /api/system/overview`
4. 建立 `GET /api/ws`
5. 用 `GET /api/system/changes` 补偿断线期间变更

有管理员权限时，可用 `GET /api/system/overview?config=1` 读取带配置的总览。

遇到 `boot_id` 变化、`config_epoch` 变化、`changes.gap=true`、WS `epoch_changed` 或 WS `resync_required` 时，应重新全量同步。

## Discovery

`GET /api/system/discovery` 是新前端、控制台、SDK 和平台接入的首个接口。

```json
{
  "data": {
    "session": {},
    "auth": {},
    "actor": {},
    "actions": [],
    "features": {},
    "security": {},
    "routes": {},
    "extensions": {}
  },
  "meta": {
    "request_id": "req_xxx",
    "generated_at": 1712345678,
    "config_epoch": "epoch_xxx"
  }
}
```

规则：

- `routes` 和 `actions` 会按当前 actor 权限、作用域和运行态能力裁剪。
- 匿名请求只看到公开入口、验证码、健康检查和 discovery。
- 受保护入口只在当前 actor 可用时发布。
- `routes.realtime_subscriptions` 只用于 `/api/ws` 的 `request.path`。
- `routes.webhooks` 对应普通 HTTP webhook 管理接口。
- `routes.captcha_new` 只在 `data.features.open_captcha=true` 时出现。

常用字段：

| 字段 | 用途 |
| --- | --- |
| `data.auth.login_delay_ms` | 登录延迟 |
| `data.auth.totp_len` | TOTP 长度 |
| `data.auth.pow_enable` | session 登录是否可能要求 PoW |
| `data.auth.allow_client_vkey_login` | 是否允许 `client_vkey` 登录 |
| `data.security.pow_bits` | PoW 位数 |
| `data.session.kind` | 当前 session 类型 |
| `data.actor.kind` | 当前 actor 类型 |

`pow_enable=true` 只表示登录链可能要求 PoW。客户端应在收到 `pow_required` 后计算并重试。

## Changes

| 参数 | 说明 |
| --- | --- |
| `after` | 上次处理到的 cursor |
| `limit` | 返回数量，默认 `100`，最大 `500` |
| `durable=1` / `history=1` | 使用持久历史窗口 |

返回字段：`cursor`、`oldest_cursor`、`next_after`、`has_more`、`gap`、`items[]`。

`items[]` 常用字段：`sequence`、`timestamp`、`name`、`resource`、`action`、`actor`、`metadata`、`fields`。

## Status 与 Usage

`status`、`registration`、`overview` 常用字段：`node_id`、`schema_version`、`api_base`、`boot_id`、`runtime_started_at`、`config_epoch`、`capabilities`、`protocol`、`counts`、`revisions`、`operations`、`idempotency`、`display`。

`usage-snapshot` 面向统计和同步判断，不等同资源详情接口。敏感字段会按 actor 权限脱敏。
