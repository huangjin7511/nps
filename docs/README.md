# NPS 文档

NPS 是一款轻量级内网穿透代理服务器。一个最小链路通常由公网 `NPS`、内网 `NPC` 和一条转发规则组成。

第一次使用时，建议先让服务端和客户端连通，再选择域名转发、TCP、UDP、代理、私密代理或 P2P 等规则。

## 快速入口

| 你要做什么 | 建议先看 |
| --- | --- |
| 完成第一次部署和验证 | [10 分钟快速开始](/getting-started/quick-start.md) |
| 选择安装方式 | [安装指南](/getting-started/install.md) |
| 启动服务端 | [启动 NPS 服务端](/getting-started/start-server.md) |
| 启动客户端 | [启动 NPC 客户端](/getting-started/start-client.md) |
| 按目标选择规则 | [规则选型总览](/guide/design/tunnel-selection.md) |
| 查询命令行参数 | [NPC 命令行参数](/reference/npc-cli.md) |
| 查询服务端配置 | [服务端配置文件](/reference/server-config.md) |
| 开发管理页面或脚本 | [管理接入入口](/reference/integration/management-api-entrypoints.md) |

## 文档分区

| 分区 | 放什么 | 入口 |
| --- | --- | --- |
| 开始使用 | 首次部署、首次连接、基础概念 | [开始使用](/getting-started/README.md) |
| 操作指南 | 已经连通后的日常操作步骤 | [操作指南](/guide/README.md) |
| 选型与规则 | 按业务目标选择转发方式 | [选型与规则](/guide/design/README.md) |
| 参考资料 | 配置字段、功能边界、FAQ | [参考资料](/reference/README.md) |
| 接口与集成 | API、平台接入、SDK、Launch 规范 | [接口与集成](/reference/integration/README.md) |

## 最短路径

1. 先看 [10 分钟快速开始](/getting-started/quick-start.md)。
2. 按你的环境选择 [安装指南](/getting-started/install.md)。
3. 启动 [NPS 服务端](/getting-started/start-server.md)。
4. 在 Web 管理界面的“客户端列表”复制快捷命令，启动 [NPC 客户端](/getting-started/start-client.md)。
5. 连通后再按目标进入 [规则选型总览](/guide/design/tunnel-selection.md)。

如果你已经知道要查的字段或接口，直接进入对应参考页，不必从快速开始重新阅读。
