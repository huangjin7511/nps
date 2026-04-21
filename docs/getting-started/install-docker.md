# Docker 安装

Docker 适合首次验证和独立部署。

## NPS 服务端

先准备宿主机配置目录：

```bash
mkdir -p /opt/nps-conf
```

启动：

```bash
docker pull duan2001/nps
docker run -d \
  --restart=always \
  --name nps \
  --net=host \
  -v /opt/nps-conf:/conf \
  -v /etc/localtime:/etc/localtime:ro \
  duan2001/nps
```

也可以把镜像名换成 `ghcr.io/djylb/nps`。

容器第一次启动会自动写默认配置。目录映射关系：

| 容器内 | 宿主机 |
| --- | --- |
| `/conf/nps.conf` | `/opt/nps-conf/nps.conf` |

修改配置后重启容器：

```bash
nano /opt/nps-conf/nps.conf
docker restart nps
```

常用字段：

| 字段 | 作用 |
| --- | --- |
| `web_username` / `web_password` | Web 管理账号密码 |
| `web_port` | Web 管理端口 |
| `bridge_tcp_port` | NPC TCP 连接端口 |
| `bridge_tls_port` | NPC TLS 连接端口 |

`--net=host` 可以避免逐个映射端口。若不用 host 网络，需要自行映射 Web、Bridge、HTTP/HTTPS、P2P 和业务隧道端口。

## NPC 客户端

首次连接建议直接传命令行参数，不先写 `npc.conf`。

```bash
docker pull duan2001/npc
docker run -d \
  --restart=always \
  --name npc \
  --net=host \
  duan2001/npc \
  -server=<server-ip>:8024 \
  -vkey=<client-vkey> \
  -type=tcp \
  -log=off
```

也可以把镜像名换成 `ghcr.io/djylb/npc`。

如果确实要使用 `npc.conf`：

```bash
mkdir -p /opt/npc-conf
docker run -d \
  --restart=always \
  --name npc \
  --net=host \
  -v /opt/npc-conf:/conf \
  duan2001/npc \
  -config=/conf/npc.conf \
  -log=off
```
