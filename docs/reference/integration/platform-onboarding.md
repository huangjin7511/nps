# 平台接入总览

本页给外部平台和多节点控制面一个最短接入路径。接口细节见 [管理接口说明](/reference/management-api.md)，配置字段见 [节点与平台对接](/reference/server-config-node.md)。

## 原则

- 节点本地数据始终是真源。
- 正式管理协议使用 `/api/` 前缀。
- `run_mode=node` 只表示开启节点控制面能力，不表示配置真源迁到平台。
- 平台应组合使用快照、增量同步和实时通道，避免频繁全量拉取。

## 连接模式

| 模式 | 适用场景 | 说明 |
| --- | --- | --- |
| `direct` | 平台可直接访问节点 Web 管理地址 | 平台主动调用节点 `/api/` |
| `reverse` | 平台无法主动连入节点，但节点可出网 | 节点主动建立 reverse WebSocket |
| `dual` | 生产环境或迁移期 | 同时保留直连和反向通道 |

`reverse` 平台 token 不能再直接访问节点 HTTP 管理接口。

## 最小配置

```ini
run_mode=node

platform_ids=main
platform_tokens=replace-with-long-random-token
platform_scopes=full
platform_enabled=true
platform_urls=https://control.example.com

platform_connect_modes=dual
platform_reverse_enabled=true
platform_reverse_ws_urls=wss://control.example.com/node/ws
platform_reverse_heartbeat_seconds=30

platform_callback_enabled=true
platform_callback_urls=https://control.example.com/node/callback
platform_callback_timeout_seconds=5
platform_callback_retry_max=2
platform_callback_retry_backoff_seconds=2
platform_callback_queue_max=100
platform_callback_signing_keys=replace-with-hmac-key

node_changes_window=1024
node_idempotency_ttl_seconds=300
node_batch_max_items=50
```

旧 `master_url` 和 `node_token` 会迁移为第一条平台配置。新部署建议直接使用多平台配置组。

## 接入顺序

1. `GET /api/system/discovery`
2. `GET /api/system/overview`
3. 如需完整管理快照，调用 `GET /api/system/export`
4. 持续消费 `GET /api/system/changes`
5. 建立 `GET /api/ws` 或启用 callback

以下情况重新全量同步：

- `boot_id` 变化。
- `config_epoch` 变化。
- `/api/system/changes` 返回 `gap=true`。
- WS 收到 `epoch_changed` 或 `resync_required`。

## 实时与补偿

| 能力 | 用途 |
| --- | --- |
| `changes` | 可补偿的增量事件窗口 |
| `changes?durable=1` | 持久历史窗口 |
| WS | 低延迟请求和事件 |
| callback | 主动推送重要事件 |
| callback queue | 失败投递查看、重放和清空 |
| idempotency | 避免重试导致重复写入 |

## 运行建议

- 每个平台使用独立 token 和签名 key。
- 优先给平台配置专用 `platform_service` 用户。
- 对外平台只开放必要作用域，默认不要给 `full`。
- 保留 callback 失败队列。
- 同时评估 `node_changes_window`、callback 队列上限和幂等 TTL。
