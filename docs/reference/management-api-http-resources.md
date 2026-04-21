# 管理接口：资源

本页列资源 CRUD 路径。新开发应使用这些 `/api/` 接口；旧页面 API 只保留兼容。

## 通用规则

| 规则 | 说明 |
| --- | --- |
| 列表参数 | 通常支持 `offset`、`limit`、`search`、`sort`、`order` |
| 请求体 | 写接口只接受 JSON object，除非接口另有说明 |
| 字段名 | 使用 canonical 字段，不依赖旧 form/query 别名 |
| 并发控制 | 更新接口可带 `expected_revision`，冲突返回 `409 revision_conflict` |
| 速率单位 | `rate_limit_total_bps` 单位是 bytes/s |
| 敏感字段 | 按 actor 权限脱敏 |

## 用户

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| `GET` | `/api/users` | 列表 |
| `GET` | `/api/users/:id` | 详情 |
| `POST` | `/api/users` | 创建 |
| `POST` | `/api/users/:id/actions/update` | 更新 |
| `POST` | `/api/users/:id/actions/status` | 启停 |
| `POST` | `/api/users/:id/actions/delete` | 删除 |

常用写字段：`username`、`password`、`totp_secret`、`expire_at`、`flow_limit_total_bytes`、`rate_limit_total_bps`、`max_connections`、`reset_flow`。

创建用户时 `password` 和 `totp_secret` 不能同时为空。状态接口 body 必须提供 `status`。

## 客户端

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| `GET` | `/api/clients` | 列表 |
| `GET` | `/api/clients/:id` | 详情 |
| `GET` | `/api/clients/:id/connections` | 同 vkey 在线实例 |
| `POST` | `/api/clients` | 创建 |
| `POST` | `/api/clients/actions/clear` | 批量清理 |
| `POST` | `/api/clients/:id/actions/update` | 更新 |
| `POST` | `/api/clients/:id/actions/ping` | 探测连通 |
| `POST` | `/api/clients/:id/actions/status` | 启停 |
| `POST` | `/api/clients/:id/actions/clear` | 清理单个客户端 |
| `POST` | `/api/clients/:id/actions/delete` | 删除 |
| `GET` | `/api/tools/qrcode` | 生成二维码 PNG |
| `POST` | `/api/tools/qrcode` | 用 JSON 生成二维码 PNG |

常用写字段：`verify_key`、`owner_user_id`、`manager_user_ids`、`expire_at`、`flow_limit_total_bytes`、`rate_limit_total_bps`、`max_connections`、`max_tunnel_num`、`reset_flow`。

补充：`verify_key` 只有具备 `clients:update` 权限的 actor 才会返回；`connections` 是在线实例运行态，不落盘；clear 支持 `flow`、`flow_limit`、`time_limit`、`rate_limit`、`conn_limit`、`tunnel_limit`。

## 隧道

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| `GET` | `/api/tunnels` | 列表 |
| `GET` | `/api/tunnels/:id` | 详情 |
| `POST` | `/api/tunnels` | 创建 |
| `POST` | `/api/tunnels/:id/actions/update` | 更新 |
| `POST` | `/api/tunnels/:id/actions/start` | 启动 |
| `POST` | `/api/tunnels/:id/actions/stop` | 停止 |
| `POST` | `/api/tunnels/:id/actions/clear` | 清理 |
| `POST` | `/api/tunnels/:id/actions/delete` | 删除 |

常用写字段：`client_id`、`port`、`server_ip`、`mode`、`target_type`、`target`、`proxy_protocol`、`local_proxy`、`auth`、`remark`、`password`、`local_path`、`strip_pre`、`enable_http`、`enable_socks5`、`entry_acl_mode`、`entry_acl_rules`、`dest_acl_mode`、`dest_acl_rules`、`expire_at`、`flow_limit_total_bytes`、`rate_limit_total_bps`、`max_connections`、`reset_flow`。

补充：`enable_http` 和 `enable_socks5` 只对 `mixProxy` 有明确意义；`password` / `auth` 只有具备 `tunnels:update` 权限的 actor 才会返回；start / stop 对 `mixProxy` 可传 `http` 或 `socks5`；clear 支持 `flow`、`flow_limit`、`time_limit`。

## 域名转发

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| `GET` | `/api/hosts` | 列表 |
| `GET` | `/api/hosts/cert-suggestion` | 可复用证书建议 |
| `GET` | `/api/hosts/:id` | 详情 |
| `POST` | `/api/hosts` | 创建 |
| `POST` | `/api/hosts/:id/actions/update` | 更新 |
| `POST` | `/api/hosts/:id/actions/start` | 启用 |
| `POST` | `/api/hosts/:id/actions/stop` | 停用 |
| `POST` | `/api/hosts/:id/actions/clear` | 清理 |
| `POST` | `/api/hosts/:id/actions/delete` | 删除 |

常用写字段：`client_id`、`host`、`target`、`proxy_protocol`、`local_proxy`、`auth`、`header`、`resp_header`、`host_change`、`remark`、`location`、`path_rewrite`、`redirect_url`、`entry_acl_mode`、`entry_acl_rules`、`scheme`、`https_just_proxy`、`tls_offload`、`auto_ssl`、`key_file`、`cert_file`、`auto_https`、`auto_cors`、`compat_mode`、`target_is_https`、`expire_at`、`flow_limit_total_bytes`、`rate_limit_total_bps`、`max_connections`、`reset_flow`、`sync_cert_to_matching_hosts`。

补充：`cert-suggestion` 支持 `host` 和 `exclude_id`；`auth` / `cert_file` / `key_file` 只有具备 `hosts:update` 权限的 actor 才会返回；start / stop 支持 `auto_https`、`auto_cors`、`compat_mode`、`https_just_proxy`、`tls_offload`、`auto_ssl`、`target_is_https`；clear 支持 `flow`、`flow_limit`、`time_limit`。

## 节点级配置与封禁

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| `GET` | `/api/settings/global` | 读取全局可修改配置 |
| `POST` | `/api/settings/global/actions/update` | 修改全局可修改配置 |
| `GET` | `/api/security/bans` | 读取封禁列表 |
| `POST` | `/api/security/bans/actions/delete` | 解除单项封禁 |
| `POST` | `/api/security/bans/actions/delete_all` | 清空全部封禁 |
| `POST` | `/api/security/bans/actions/clean` | 清理过期封禁 |

当前 `settings/global` 主要包含节点级入口 ACL，后续可能扩展更多运行时可修改配置。`security/bans/actions/delete` 的 body 需要 `key`。
