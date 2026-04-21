# 接口与集成

这一部分收纳对外接口、平台接入、程序集成和启动协议。普通单节点部署通常不需要先看这里。

## 按任务进入

| 你要做什么 | 建议页面 |
| --- | --- |
| 开发新的本地管理前端、控制台或脚本 | [管理接入入口](/reference/integration/management-api-entrypoints.md) |
| 对接多节点平台或外部控制面 | [平台接入总览](/reference/integration/platform-onboarding.md) |
| 查询正式管理接口规则 | [管理接口说明](/reference/management-api.md) |
| 查询 HTTP 路径目录 | [HTTP 接口目录](/reference/management-api-http.md) |
| 使用 WebSocket、reverse WS 或 callback | [实时通道与回调](/reference/management-api-realtime.md) |
| 评估客户端程序内集成 | [NPC SDK 状态](/reference/npc-sdk.md) |
| 使用 `-launch`、`npc://`、远程源或多实例 | [NPC Launch 规范](/reference/npc-launch.md) |

## 选择建议

新的管理页面优先从 discovery 发现路径和能力，不建议按旧 Web 页面接口开发。旧接口只用于兼容现有页面或历史脚本。
