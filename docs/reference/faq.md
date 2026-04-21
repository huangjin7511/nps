# FAQ

常见问题先看这里。需要精确配置项时，再看 [服务端配置文件](/reference/server-config.md)。

## 服务端无法启动

先检查：

1. 端口是否冲突，示例配置常用 `8024`、`8081`、`80`、`443`、`6000`。
2. 配置目录下是否有完整的 `conf/` 和 `web/`。
3. 证书路径、日志路径和自定义目录是否有效。

排查时可停止服务后前台运行一次：

```bash
nps -conf_path=/etc/nps
```

发布包目录内直接运行时用：

```bash
./nps
```

## 修改配置后没有生效

先确认改的是实际生效的配置文件：

| 场景 | 常见配置文件 |
| --- | --- |
| Linux / macOS 标准安装 | `/etc/nps/conf/nps.conf` |
| Windows 管理员安装 | `C:\Program Files\nps\conf\nps.conf` |
| Windows 无管理员脚本安装 | `%LOCALAPPDATA%\nps\conf\nps.conf` |
| 自定义目录 | `-conf_path` 指向目录下的 `conf/nps.conf` |

多数配置修改后需要重启。`nps reload` 只适合非 Windows 平台的少量运行态配置，并且需要能定位 `nps.pid`。端口、Bridge、HTTPS 入口或运行模式变更应直接重启。

## 客户端无法连接服务端

按顺序检查：

1. 服务端防火墙和云安全组是否放行桥接端口。
2. `server` 地址和端口是否正确。
3. `vkey` 是否对应当前客户端。
4. 连接协议是否匹配，例如 TLS 端口要配 `-type=tls`。
5. 客户端和服务端版本是否兼容。

## 客户端在线但业务不通

“NPC 在线”和“业务可访问”不是同一件事。继续检查：

1. 是否创建了隧道或域名转发规则。
2. NPC 所在机器能否访问目标地址。
3. NPS 公网入口端口是否开放。
4. 目标服务是否正常监听。

## Docker 隧道端口连不上

如果 NPS 容器没有使用 `--net=host`，需要把 NPS 监听的端口映射出来。要暴露的是 NPS 容器端口，不是 NPC 容器端口。

## P2P 直连失败

P2P 成功率取决于 NAT 和网络环境。双方都是 Symmetric NAT 时通常无法直连。稳定优先时，使用 [私密代理](/guide/tunnels/secret.md) 或开启回退。

## 一条命令连接多个服务端

多个值用逗号一一对应：

```bash
npc -server=xxx:123,yyy:456 -vkey=key1,key2 -type=tcp,tls
```

建议 `server`、`vkey`、`type` 数量保持一致。

## 真实 IP、反向代理和 HTTPS

- HTTP / HTTPS 域名转发传真实 IP：启用 `http_add_origin_header=true`，后端读取 `X-Forwarded-For` 或 `X-Real-IP`。
- TCP / UDP 传来源地址：后端支持时使用 Proxy Protocol。
- Nginx / Caddy 放在 NPS 前面：让前置代理监听 `80/443`，转发到 NPS 本地端口，并传递 `X-NPS-Http-Only`、`X-Real-IP`、`X-Forwarded-For`。
- 后端自己处理 HTTPS：域名转发可选择由后端处理 HTTPS，NPS 只转发。

## 日志在哪里

| 程序 | Linux / macOS | Windows |
| --- | --- | --- |
| NPS | `/var/log/nps.log` | `nps.exe` 所在目录的 `nps.log` |
| NPC | `/var/log/npc.log` | `npc.exe` 所在目录的 `npc.log` |

如果显式设置了 `log_path`，以配置值为准。

## 旧客户端接入新服务端

仅在必须兼容旧环境时使用：

1. 服务端设置 `secure_mode=false`。
2. 旧客户端增加 `-proto_version=0`。

新部署建议保持 `secure_mode=true`。

## 到期时间限制

先在 `nps.conf` 启用：

```ini
allow_time_limit=true
```

然后在 Web 管理界面为客户端或资源设置到期时间。

## 还查不到答案

继续看 [功能清单](/reference/features.md)、[服务端配置文件](/reference/server-config.md) 和 [补充说明](/reference/notes.md)。仍无法定位时，到 [GitHub Issues](https://github.com/djylb/nps/issues) 提交问题。
