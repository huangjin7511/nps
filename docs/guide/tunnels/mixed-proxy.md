# 混合代理

混合代理用于把“访问内网的能力”暴露为代理入口。它不是 VPN，只有显式配置代理的应用流量才会经过 NPS。

## 什么时候用

- 通过代理访问多个内网目标。
- 浏览器、开发工具或系统代理需要统一入口。
- 需要限制代理可访问的目标。

## 模式

| 模式 | 说明 |
| --- | --- |
| HTTP 代理 | 适合浏览器和只支持 HTTP 代理的程序 |
| Socks5 代理 | 更通用 |
| 混合代理 | 同一端口同时提供 HTTP 和 Socks5 |

当前推荐统一使用 `mode=mixProxy`，再通过 `http_proxy` / `socks5_proxy` 控制能力。旧 `mode=httpProxy`、`mode=socks5` 仍可解析。

## Web 界面字段

| 字段 | 作用 |
| --- | --- |
| 监听端口 | 公网代理端口 |
| HTTP 代理 | 是否启用 HTTP 代理 |
| Socks5 代理 | 是否启用 Socks5 代理 |
| 认证信息 | 代理用户名和密码 |
| 目标 ACL | 限制可访问目标 |

## 使用示例

Socks5：

1. 创建混合代理规则。
2. 只启用 SOCKS5。
3. 监听端口填 `8003`。
4. 外部设备设置 SOCKS5 代理为 `1.1.1.1:8003`。

HTTP：

1. 创建混合代理规则。
2. 只启用 HTTP。
3. 监听端口填 `8004`。
4. 外部设备设置 HTTP 代理为 `1.1.1.1:8004`。

## `npc.conf`

混合代理：

```ini
[mix]
mode=mixProxy
server_port=19009
http_proxy=true
socks5_proxy=true
multi_account=conf/multi_account.conf
```

仅 Socks5：

```ini
[socks5]
mode=mixProxy
server_port=19009
http_proxy=false
socks5_proxy=true
```

仅 HTTP：

```ini
[http]
mode=mixProxy
server_port=19004
http_proxy=true
socks5_proxy=false
```

## 目标 ACL

目标 ACL 用于限制代理能访问哪里。示例：

- `1.2.3.4`
- `10.0.0.0/8`
- `full:example.com`
- `*.example.org`
- `keyword:intranet`
- `geoip:private`
- `geosite:cn`

规则：

- IP 规则对出站链路通用。
- 域名规则、`geosite:xx`、域名通配主要用于混合代理。
- 用户目标 ACL 与隧道目标 ACL 会叠加。
- 启用任一目标 ACL 后，Socks5 UDP ASSOCIATE 会被拒绝。

## 注意

- 建议开启用户名密码认证。
- 没有配置代理的应用不会自动走这条链路。
- 只暴露单个业务服务时，优先用 TCP、UDP 或域名转发。
