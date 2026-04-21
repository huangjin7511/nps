# 参考资料

这一部分适合已经知道要查什么，只需要准确字段、行为边界或排查入口的场景。

如果还没有完成首次部署和连接验证，先看 [开始使用](#/getting-started/README)。API、SDK、平台接入和启动协议放在 [接口与集成](#/reference/integration/README)。

## 配置与运行

| 页面 | 内容 |
| --- | --- |
| [NPC 命令行参数](#/reference/npc-cli) | `npc` 参数、默认值和常见组合 |
| [服务端配置文件](#/reference/server-config) | `nps.conf` 主题入口 |
| [基础项与密钥](#/reference/server-config-basics) | 基础运行项、密钥和路径规则 |
| [Web、HTTP 与安全](#/reference/server-config-web) | Web 管理端、登录保护、真实 IP 与前置代理 |
| [入口端口与桥接](#/reference/server-config-ports) | `bridge_*`、HTTP / HTTPS 入口、P2P 入口 |
| [节点与平台对接](#/reference/server-config-node) | `run_mode=node`、多平台、reverse 和 callback |
| [访问控制与运行](#/reference/server-config-runtime) | ACL、日志、限制开关与调试 |

## 功能与排查

| 页面 | 内容 |
| --- | --- |
| [功能清单与扩展能力](#/reference/features) | 能力总入口 |
| [传输与连接](#/reference/features-transport) | 压缩、加密、KCP、多路复用和断线判定 |
| [站点与 HTTP](#/reference/features-http) | 站点能力总入口 |
| [代理、转发与路由](#/reference/features-routing) | 嵌套转发、端口映射和端口复用 |
| [访问控制与限制](#/reference/features-access) | ACL、流量、带宽、连接数和 IP 限制 |
| [运维与调试](#/reference/features-ops) | 环境变量、健康检查、日志和 pprof |
| [FAQ](#/reference/faq) | 常见问题 |
| [补充说明](#/reference/notes) | 兼容和历史说明 |
