# HTTPS 与反向代理

本页说明 HTTPS 链路怎么选、证书放哪里、前置代理怎么配。

## 模式选择

| 目标 | 关键开关 | NPS 是否解密 TLS | 后端接收 |
| --- | --- | --- | --- |
| 后端自己处理证书，NPS 只按 SNI 分流 | `https_just_proxy=true` | 否 | 原始 TLS |
| NPS 处理 TLS，再转发明文 | `tls_offload=true` | 是 | 明文 HTTP 或 TCP |
| NPS 终止 HTTPS，再代理到 HTTPS 后端 | `target_is_https=true` | 是 | HTTPS |
| HTTP 自动跳 HTTPS | `auto_https=true` | 否 | 重定向 |

`auto_https` 只做跳转。证书来源由 `auto_ssl`、手工证书或默认证书决定。

## 证书

手工证书：在域名转发里填写或上传证书与私钥。

自动证书：

```text
auto_ssl=true
```

前提：域名已解析到 NPS，`80` 或 `443` 可用于验证。需要强制申请时设置 `force_auto_ssl=true`。

默认证书：

```ini
https_default_cert_file=conf/server.pem
https_default_key_file=conf/server.key
```

## Web 管理 HTTPS

```ini
web_open_ssl=true
web_cert_file=conf/server.pem
web_key_file=conf/server.key
```

访问：

```text
https://<server-ip>:<web-port>
```

管理后台挂到子路径时配置：

```ini
web_base_url=/nps
```

## 前置 Nginx / Caddy

适合把公网 `80/443` 交给 Nginx、Caddy、WAF 或统一网关。NPS 可只监听本地 HTTP 入口：

```ini
http_proxy_port=8010
x_nps_http_only=password
allow_x_real_ip=true
trusted_proxy_ips=127.0.0.1
```

Nginx 最小示例：

```nginx
server {
    listen 80;
    server_name _;
    location / {
        proxy_pass http://127.0.0.1:8010;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $http_connection;
        proxy_set_header Host $http_host;
        proxy_set_header X-NPS-Http-Only "password";
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_redirect off;
        proxy_buffering off;
    }
}
```

`X-NPS-Http-Only` 用于识别可信前置代理，避免重复 HTTP 到 HTTPS 跳转。`allow_x_real_ip` 和 `trusted_proxy_ips` 用于管理后台来源识别。

## 真实 IP

业务 HTTP / HTTPS 反代优先使用：

```ini
http_add_origin_header=true
```

后端支持 Proxy Protocol 时，也可以在目标上启用 Proxy Protocol，适用于 TCP、UDP 和域名转发。

## 代理到 NPS 本机服务

```ini
allow_local_proxy=true
```

然后域名转发目标可指向 `127.0.0.1:<port>`。
