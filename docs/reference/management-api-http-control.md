# 管理接口：控制与示例

本页列控制类接口。资源 CRUD 见 [资源接口](/reference/management-api-http-resources.md)。

## 控制接口

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| `POST` | `/api/batch` | 批量执行子请求 |
| `POST` | `/api/traffic` | 写入流量增量，仅 `full` 管理视角 |
| `POST` | `/api/clients/actions/kick` | 踢断客户端 |
| `POST` | `/api/system/actions/sync` | 重载运行态，仅 `full` 管理视角 |
| `GET` | `/api/system/export` | 导出完整业务配置 |
| `POST` | `/api/system/import` | 导入完整业务配置 |
| `GET` | `/api/callbacks/queue` | 查看 callback 失败队列 |
| `POST` | `/api/callbacks/queue/actions/replay` | 重放 callback 队列 |
| `POST` | `/api/callbacks/queue/actions/clear` | 清空 callback 队列 |
| `GET` | `/api/webhooks` | webhook 列表 |
| `POST` | `/api/webhooks` | 创建 webhook |
| `GET` | `/api/webhooks/{id}` | webhook 详情 |
| `POST` | `/api/webhooks/{id}/actions/update` | 更新 webhook |
| `POST` | `/api/webhooks/{id}/actions/status` | 启停 webhook |
| `POST` | `/api/webhooks/{id}/actions/delete` | 删除 webhook |

## 写请求规则

- 建议写请求带 `Idempotency-Key` 或 `X-Idempotency-Key`。
- 建议写请求带 `X-Operation-ID`，再用 `/api/system/operations?operation_id=...` 查询摘要。
- 重复幂等命中时，响应带 `X-Idempotent-Replay: true`。
- 除 `/api/batch` 外，写接口只接受 JSON object body。
- `client_id` 和 `verify_key` 同时提交时，必须指向同一客户端。

## Batch

`POST /api/batch` 支持 `{"items":[...]}` 或顶层数组。

```json
{
  "items": [
    {
      "method": "GET",
      "path": "/api/clients"
    },
    {
      "method": "GET",
      "path": "/api/system/status"
    }
  ]
}
```

限制：

- 子请求只支持 `GET` 和 `POST`。
- 子请求路径使用正式 HTTP 管理路径。
- 不能嵌套 `/api/batch`。
- 不能转发 `/api/ws`。
- 不能转发 `/api/realtime/subscriptions...`。

## 示例

读取客户端列表：

```bash
curl -H "X-Node-Token: <platform_token>" \
  "http://127.0.0.1:8081/api/clients?offset=0&limit=20"
```

新增客户端：

```bash
curl -X POST \
  -H "X-Node-Token: <platform_token>" \
  -H "Content-Type: application/json" \
  -d "{\"verify_key\":\"demo-key\",\"remark\":\"demo\"}" \
  http://127.0.0.1:8081/api/clients
```

导出或导入配置：

```bash
curl -H "X-Node-Token: <platform_token>" \
  http://127.0.0.1:8081/api/system/export
```

```bash
curl -X POST \
  -H "X-Node-Token: <platform_token>" \
  -H "Content-Type: application/json" \
  --data-binary @node-config-export.json \
  http://127.0.0.1:8081/api/system/import
```

## 边界

- 导入导出只处理业务配置和业务数据，不处理 changes、幂等缓存、callback 队列等协议辅助运行态。
- 导入成功后会切换 `config_epoch`，旧 changes cursor、旧幂等缓存、旧实时会话都会失效。
- 节点负责本地强约束：`flow_limit_total_bytes`、`expire_at`、`max_clients`、`max_tunnels`、`max_hosts`。
- 外部平台负责跨节点总量策略。`rate_limit_total_bps` 和 `max_connections` 不适合做跨节点强约束。
