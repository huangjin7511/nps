# NPC Launch 规范

这一组页面说明 `-launch`、`npc://`、远程 URL、JSON 启动描述和多实例的正式规范。

如果你只是想快速上手 `-launch`，先看 [快速启动与远程源](/guide/client/launch.md)。

这一组页面属于 [接口与集成](/reference/integration/README.md) 部分。

## 先按问题找

| 你要确认什么 | 建议页面 |
| --- | --- |
| `-launch` 是什么、冻结规则、解析顺序和多 payload | [基础规则与解析顺序](/reference/npc-launch-basics.md) |
| `npc://` 明文直连、base64 包装和哪些写法不算规范 | [`npc://` 协议](/reference/npc-launch-uri.md) |
| JSON 顶层结构、`direct`、`config`、`local` 和 `profiles` | [JSON 启动描述](/reference/npc-launch-json.md) |
| 远程源状态语义、兼容边界和推荐实践 | [远程源与兼容性](/reference/npc-launch-remote.md) |

## 推荐阅读顺序

1. [基础规则与解析顺序](/reference/npc-launch-basics.md)
2. [`npc://` 协议](/reference/npc-launch-uri.md)
3. [JSON 启动描述](/reference/npc-launch-json.md)
4. [远程源与兼容性](/reference/npc-launch-remote.md)
