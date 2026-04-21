# P2P 隧道

P2P 隧道会尽量让访问端和目标端直连，适合大流量、低延迟场景。网络不支持直连时，可按配置回退到私密代理。

## 什么时候用

- 游戏联机。
- 大文件传输。
- 低延迟点对点访问。
- 希望减少服务器中转流量。

P2P 不是创建服务端规则后就自动可用。访问端仍要启动一个 NPC，本地监听端口，再通过本地端口访问远端服务。

## 准备

1. NPS 必须有公网 IP。
2. 在 `nps.conf` 中启用 `p2p_port`。
3. 防火墙放行 `p2p_port` 到 `p2p_port + 2` 的 UDP 端口。

示例：

```ini
p2p_ip=0.0.0.0
p2p_port=6000
```

如果 `p2p_port=6000`，通常放行 `6000/udp`、`6001/udp`、`6002/udp`。

## 最小示例

假设目标服务是 `10.2.50.2:22`，P2P 密钥是 `p2pssh`。在 Web 界面创建 P2P 代理后，访问端执行：

```bash
./npc -server=1.1.1.1:8024 -vkey=123 -type=tcp -password=p2pssh -target=10.2.50.2:22 -target_type=tcp
```

默认监听本地 `2000` 端口：

```bash
ssh -p 2000 root@127.0.0.1
```

## `npc.conf`

提供服务的一侧：

```ini
[ssh_p2p]
mode=p2p
password=ssh3
```

访问端：

```ini
[p2p_ssh]
local_port=2002
password=ssh3
target_addr=10.2.50.2:22
target_type=tcp
fallback_secret=true
```

访问端 section 名以 `p2p` 开头且未写 `mode` 时，NPC 会按本地 P2P 模式解析。

## 访问端命令

Web 管理界面会按用途生成命令，优先复制页面命令。

| 用途 | 参数 |
| --- | --- |
| TCP / UDP 访问 | 指定 `-target`，或使用默认 `local_type=p2p` |
| HTTP / Socks5 代理访问 | 使用 `-local_type=p2ps` |
| 透明代理访问 | 使用 `-local_type=p2pt` |

`-target` 或 `target_addr` 填最终要访问的真实服务地址，不是 NPS 公网地址。`target_type` 可写 `tcp`、`udp` 或 `all`，留空时按 `all` 处理。

`fallback_secret=true` 表示打洞失败后回退到 secret 中转。代理类本地模式要在回退后保持动态目标语义，服务端对应隧道不要配置固定 `target`。

## 成功率

- 双方都是 Symmetric NAT 时通常无法直连。
- NPC 与 NPS 不应处于同一内网环境下作为标准公网 P2P 测试。
- 不同运营商、路由器和 NAT 类型都会影响结果。
- 稳定优先时，使用 [私密代理](/guide/tunnels/secret.md) 或保留 `fallback_secret=true`。

## 参数

常用参数：

- `-p2p_type`
- `-fallback_secret`
- `-disable_p2p`

`-p2p_timeout` 是兼容保留参数，不建议作为通用 P2P 超时开关。服务端 P2P 探测、握手和传输超时优先看 `p2p_*_timeout_ms` 配置。
