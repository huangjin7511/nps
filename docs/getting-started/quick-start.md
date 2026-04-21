# 10 分钟快速开始

目标：启动 NPS，连上一台 NPC，再创建一条公网可访问的 TCP 隧道。

关系：`NPS` 在公网，`NPC` 在内网，外部访问先到 `NPS`，再转到真实内网服务。

## 准备

- 一台可公网访问的服务器。
- 一台能访问 NPS 连接端口的内网机器。
- 一个已存在的内网服务，例如 `127.0.0.1:8080` 或 `10.0.0.10:22`。

第一次验证建议用 TCP 隧道，不要先配置域名、HTTPS、P2P 或前置代理。

## 1. 启动 NPS

Docker 示例：

```bash
mkdir -p /opt/nps-conf
docker pull duan2001/nps
docker run -d --restart=always --name nps --net=host -v /opt/nps-conf:/conf -v /etc/localtime:/etc/localtime:ro duan2001/nps
```

容器第一次启动会自动写默认配置。容器内 `/conf/nps.conf` 对应宿主机 `/opt/nps-conf/nps.conf`。

编辑配置并重启：

```bash
nano /opt/nps-conf/nps.conf
docker restart nps
```

至少确认：

| 字段 | 作用 |
| --- | --- |
| `web_port` | Web 管理端口，示例值 `8081` |
| `bridge_tcp_port` | NPC TCP 连接端口，示例值 `8024` |
| `bridge_tls_port` | NPC TLS 连接端口，示例值 `8025` |
| `web_username` / `web_password` | Web 管理账号密码 |

其他安装方式见 [安装指南](/getting-started/install.md)。

## 2. 登录 Web 管理端

浏览器访问：

```text
http://<server-ip>:8081
```

示例账号密码：

```text
admin / 123
```

正式环境应修改 `nps.conf` 中的 `web_username` 和 `web_password`，再重启 NPS。

## 3. 启动 NPC

推荐直接复制 Web 管理界面“客户端列表”提供的快捷命令。

```bash
./npc -server=<server-ip>:8024 -vkey=<client-vkey> -type=tcp
```

如果使用 TLS 连接：

```bash
./npc -server=<server-ip>:8025 -vkey=<client-vkey> -type=tls
```

第一次连接不要直接执行不带参数的 `npc`，也不要先改 `npc.conf`，避免误读默认配置。

## 4. 创建 TCP 隧道

在 Web 管理界面找到在线客户端，创建 TCP 隧道。

| 用途 | 监听端口 | 内网目标 | 外部访问 |
| --- | --- | --- | --- |
| 网站 | `10080` | `127.0.0.1:8080` | `http://<server-ip>:10080` |
| SSH | `10022` | `127.0.0.1:22` | `ssh -p 10022 <user>@<server-ip>` |

## 检查

- Web 管理端可以登录。
- 客户端显示在线。
- 公网端口可以访问内网目标。

如果隧道不通，先检查 NPC 所在机器能否访问内网目标，以及服务端防火墙是否放行新端口。
