# NPC 命令行参数

本页用于查询 `npc` 参数。第一次连接优先复制 Web 管理界面“客户端列表”提供的启动命令。

示例中的 `./npc` 表示从发布包目录执行；已加入系统路径时可直接用 `npc`。

## 常用参数

```bash
./npc -server=127.0.0.1:8024 -vkey=YOUR_CLIENT_VKEY -type=tcp
```

| 参数 | 说明 | 默认值 |
| --- | --- | --- |
| `-server` | NPS 地址，支持 `addr:port[@host_or_sni[:port]][/path]` | 无 |
| `-vkey` | 客户端认证密钥 | 无 |
| `-type` | 连接方式：`tcp` / `tls` / `kcp` / `quic` / `ws` / `wss` | `tcp` |
| `-config` | 配置文件路径，可逗号分隔多份 | 自动回退默认路径 |
| `-launch` | 启动描述，支持 `npc://`、URL、base64、JSON，可重复 | 无 |
| `-proxy` | 通过 `socks5://`、`http://`、`https://` 代理连接 | 无 |
| `-local_ip` | 出站绑定本地 IP，可逗号分隔 | 无 |
| `-local_ip_forward` | 让 `local_ip` 作用于隧道转发出口 | `false` |
| `-debug` | 调试模式 | `true` |
| `-log` | `stdout` / `file` / `both` / `off` | `file` |
| `-log_path` | 日志路径 | 自动推导 |
| `-log_level` | `trace` 到 `off` | `trace` |
| `-log_compress` | 压缩日志 | `false` |
| `-log_max_days` | 日志保留天数 | `7` |
| `-log_max_files` | 最大日志文件数 | `10` |
| `-log_max_size` | 单个日志文件大小，MB | `5` |
| `-log_color` | 控制台彩色输出 | `true` |
| `-auto_reconnect` | 断线自动重连 | `true` |
| `-disconnect_timeout` | 控制连接断线超时，秒 | `30` |
| `-keepalive` | 保活周期，秒 | `0` |
| `-pprof` | PProf 地址，格式 `ip:port` | 无 |
| `-local_type` | `secret` / `p2p` / `p2ps` / `p2pt` | `p2p` |
| `-local_port` | 本地监听端口 | `2000` |
| `-password` | 本地访问模式密钥 | 无 |
| `-target` | 本地访问目标 | 无 |
| `-target_type` | `all` / `tcp` / `udp` | `all` |
| `-local_proxy` | 本地访问模式允许代理到本机目标 | `false` |
| `-p2p_type` | P2P 通道：`quic` / `kcp` | `quic` |
| `-p2p_timeout` | 兼容保留参数 | `5` |
| `-disable_p2p` | 禁用 P2P | `false` |
| `-fallback_secret` | P2P 失败回退到 secret | `true` |
| `-proto_version` | 协议版本索引 | 最新 |
| `-skip_verify` | 跳过服务端证书指纹校验 | `false` |
| `-dns_server` | DNS 服务器 | `8.8.8.8` |
| `-ntp_server` | NTP 服务器 | 无 |
| `-ntp_interval` | NTP 最小查询间隔，分钟 | `5` |
| `-timezone` | 时区 | 无 |
| `-time` | 注册 IP 有效时间，小时 | `2` |
| `-gen2fa` | 生成 TOTP 密钥 | `false` |
| `-get2fa` | 输出一次 TOTP 验证码 | 无 |
| `-tls_enable` | 旧兼容参数，等同切到 `tls` | `false` |
| `-version` | 显示版本 | 无 |

默认配置路径：

| 系统 | 路径 |
| --- | --- |
| Linux / macOS | 当前工作目录的 `conf/npc.conf` |
| Windows | `npc.exe` 同目录的 `conf\npc.conf` |

## 常见组合

前台直连：

```bash
./npc -server=127.0.0.1:8024 -vkey=YOUR_CLIENT_VKEY -type=tcp -log=stdout -debug=false
```

配置文件：

```bash
./npc -config=/path/to/npc.conf -log=off
```

通过代理接入：

```bash
./npc -server=127.0.0.1:8024 -vkey=YOUR_CLIENT_VKEY -proxy=socks5://127.0.0.1:1080
```

多个服务端：

```bash
./npc -server=10.0.0.1:8024,10.0.0.2:8025 -vkey=key1,key2 -type=tcp,tls
```

## 建议

- 第一次连通时显式写 `-server`、`-vkey`、`-type`。
- 需要配置文件时显式写 `-config=...`，不要依赖默认回退。
- 需要远程源、多实例或统一启动描述时，再看 [NPC Launch](/reference/npc-launch.md)。
- `-p2p_timeout` 和 `-tls_enable` 属于兼容或排查参数，新配置通常不用。
