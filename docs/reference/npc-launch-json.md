# NPC Launch：JSON 启动描述

本页说明 `-launch` JSON 结构。基础用法见 [NPC Launch](/reference/npc-launch.md)。

## 顶层结构

| 字段 | 用途 |
| --- | --- |
| `version` | 格式版本，可省略 |
| `runtime` | 进程级全局参数 |
| `direct` | 模拟命令行直连 |
| `config` | 模拟 `npc.conf` |
| `local` | 模拟 `p2p` / `secret` 本地模式 |
| `profiles` | 一份 JSON 启动多个实例 |

顶层必须且只能出现 `direct`、`config`、`local`、`profiles` 其中一种。`nodes` 是 `profiles` 的兼容别名。`runtime` 是进程级参数。

## Runtime

常用字段：`log`、`log_level`、`log_path`、`debug`、`proto_version`、`skip_verify`、`keepalive`、`dns_server`、`ntp_server`、`ntp_interval`、`timezone`、`disable_p2p`、`p2p_type`、`local_ip_forward`、`auto_reconnect`、`disconnect_timeout`、`p2p_timeout`。

`p2p_timeout` 是兼容字段，不建议作为通用 P2P 超时开关。常规 P2P 超时优先使用服务端 `p2p_*_timeout_ms`。

## Direct

`direct` 对应命令行直连。一条 `direct` 可用数组表达多个节点连接。

```json
{
  "direct": {
    "server": ["10.0.0.1:8024/ws"],
    "vkey": ["node-a"],
    "type": ["ws"]
  }
}
```

常用字段：`server`、`vkey`、`type`、`proxy`、`local_ip`。

## Config

`config` 用于表达 `npc.conf`，适合需要完整控制客户端配置的场景。

| 字段 | 用途 |
| --- | --- |
| `source` | 本地配置文件路径或 HTTP(S) 配置 URL |
| `common` | `[common]` 配置 |
| `hosts` | 域名转发配置 |
| `tasks` | 隧道配置 |
| `healths` | 健康检查配置 |
| `local_servers` | 本地 visitor / secret 配置 |

兼容别名：`tunnels -> tasks`、`locals -> local_servers`、`common.server -> common.server_addr`、`common.type -> common.conn_type`、`common.proxy -> common.proxy_url`。

`config.source` 可指向本地配置文件或返回配置内容的 URL。内联字段与 `source` 同时存在时，`common` 按字段覆盖，列表类字段追加。

结构化字段名支持 `_`、`-`、`.` 三种分隔方式，例如 `server_addr`、`server-addr`、`server.addr` 视为同一字段。

```json
{
  "config": {
    "common": {
      "server_addr": "127.0.0.1:8024/ws",
      "vkey": "demo",
      "conn_type": "ws"
    },
    "tasks": [
      {
        "mode": "tcp",
        "server_port": "10080",
        "target_addr": ["127.0.0.1:80"]
      }
    ]
  }
}
```

## Local

`local` 对应 `p2p` / `secret` 本地模式。常用字段：`server`、`vkey`、`type`、`proxy`、`local_ip`、`local_type`、`local_port`、`password`、`target`、`target_addr`、`target_type`、`fallback_secret`、`local_proxy`。

## Profiles

`profiles` 用于一份 launch 启动多个实例。每个 profile 内再写 `direct`、`config` 或 `local`。

```json
{
  "profiles": [
    {
      "name": "edge",
      "direct": {
        "server": ["10.0.0.1:8024/ws"],
        "vkey": ["edge-a"],
        "type": ["ws"]
      }
    }
  ]
}
```

也可以重复传入多个 `-launch`，CLI 会归并成 `profiles`。长期保存时建议直接写规范 JSON。
