# 功能清单：访问控制与限制

本页列 ACL、IP 限制、代理目标限制和配额能力。

## 目标 ACL

混合代理支持目标黑白名单，启用后 Socks5 不支持 UDP 代理。

| 规则类型 | 示例 |
| --- | --- |
| IP | `1.2.3.4`、`10.0.0.0/8`、`10.*`、`geoip:private` |
| 域名 | `full:example.com`、`*.example.com`、`keyword:intranet`、`regexp:...`、`geosite:cn` |

IP 规则对出站链路通用。域名规则主要用于混合代理。用户目标 ACL 与隧道目标 ACL 会叠加，任一侧拒绝即拒绝。

## 配额限制

| 能力 | 启用配置 | 行为 |
| --- | --- | --- |
| 流量限制 | `allow_flow_limit=true` | 达到上限后拒绝新访问，域名转发返回 `404` |
| 带宽限制 | `allow_rate_limit=true` | 限制入口与出口总速率 |
| 时间限制 | `allow_time_limit=true` | 到期后拒绝新访问 |
| 最大连接数 | `allow_connection_num_limit=true` | 限制单个客户端长连接数量 |
| 最大隧道数 | `allow_tunnel_num_limit=true` | 限制单个客户端可创建隧道数量 |

时间限制留空表示不限制，支持日期格式和时间戳，解析受系统时区影响。

## 来源 IP ACL

来源访问控制分两类：

| 类型 | 说明 |
| --- | --- |
| 静态来源 ACL | 全局、用户、客户端、隧道、域名转发均支持关闭、白名单、黑名单 |
| 动态登录封禁 | 按用户名 / IP 统计登录失败次数 |

来源 ACL 支持精确 IP、CIDR、IPv4 前缀通配符、`geoip:xx`、`geoip:private`。判定顺序为 `global -> user -> client -> tunnel/host`。

## 端口白名单

用 `allow_ports` 限制可开放的公网端口。留空表示不限制。

```ini
allow_ports=9001-9009,10001,11000-12000
```

## IP 访问登记

启用：

```ini
ip_limit=true
```

客户端登记当前公网 IP：

```bash
./npc register -server=ip:port -vkey=PUBLIC_KEY_OR_CLIENT_VKEY -time=2
```

`time` 单位是小时。通过管理接口认证后，当前来源 IP 也会自动获得 2 小时访问权限，并在后续有效管理请求中续期。

公网 IP 可能变化，多个设备也可能共用同一个公网 IP，请合理设置有效期。过期记录会自动清理。
