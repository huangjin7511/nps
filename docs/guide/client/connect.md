# 客户端连接与配置

本页说明 NPC 如何连接 NPS，以及什么时候使用命令行、系统服务、配置文件或 `-launch`。

第一次启动只建议看前两节：先命令行连通，再决定是否注册为服务。

## 1. 第一次连接

最稳妥的方式：

1. 登录 Web 管理界面
2. 进入“客户端列表”
3. 找到客户端并展开详情
4. 复制页面里的 TCP、TLS、KCP、QUIC、WS 或 WSS 启动命令

TCP 示例：

```bash
./npc -server=<server-ip>:8024 -vkey=<client-vkey> -type=tcp
```

Windows：

```powershell
npc.exe -server="<server-ip>:8024" -vkey="<client-vkey>" -type="tcp"
```

TLS 示例：

```bash
./npc -server=<server-ip>:8025 -vkey=<client-vkey> -type=tls
```

连接多个服务端：

```bash
./npc -server=<server-a>:8024,<server-b>:8025 -vkey=<vkey-a>,<vkey-b> -type=tcp,tls
```

要点：

| 项 | 说明 |
| --- | --- |
| `-server` | NPS 连接地址 |
| `-vkey` | 客户端密钥 |
| `-type` | NPC 到 NPS 的连接协议，不是隧道类型 |

不要第一次就直接执行无参数 `npc`。源码会在缺少 `-server`、`-vkey`、`-launch` 等显式参数时回退读取默认 `conf/npc.conf`。

## 2. 注册为系统服务

先前台确认连通，再注册服务。

Linux / macOS：

```bash
sudo npc install -server=<server-ip>:8024 -vkey=<client-vkey> -type=tcp -log=off
sudo npc start
```

Windows：

```powershell
npc.exe install -server="<server-ip>:8024" -vkey="<client-vkey>" -type="tcp" -log="off"
npc.exe start
```

变更连接参数时，先卸载再重新安装：

```bash
sudo npc uninstall
```

```powershell
npc.exe uninstall
```

## 3. 配置文件模式

这是进阶用法。确认命令行直连正常后，再考虑 `npc.conf`。

适合：

- 一份配置管理多个隧道
- 使用文件隧道
- 使用本地 `secret` / `p2p` 访问模式

显式指定配置文件：

```bash
./npc -config=/path/to/npc.conf
```

多配置：

```bash
./npc -config=/path/to/npc1.conf,/path/to/npc2.conf
```

不要依赖默认回退路径。

## 4. `-launch`

`-launch` 适合远程源、JSON、多实例或统一分发启动描述。实际使用见 [快速启动与远程源](/guide/client/launch.md)，字段规范见 [NPC Launch 规范](/reference/npc-launch.md)。
