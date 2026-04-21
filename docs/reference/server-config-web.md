# 服务端配置：Web、HTTP 与安全

本页列 Web 管理、登录保护、真实 IP 和前置代理相关配置。端口配置见 [入口端口与桥接](/reference/server-config-ports.md)。

## Web 管理

| 名称 | 说明 |
| --- | --- |
| `web_port` | Web 管理端口，示例配置常见为 `8081` |
| `web_ip` | Web 监听地址 |
| `web_host` | 端口复用时访问管理页面的域名 |
| `web_username` | 管理员账号，示例值为 `admin` |
| `web_password` | 管理员密码，示例值为 `123` |
| `web_open_ssl` | 是否启用 Web HTTPS |
| `web_cert_file` | Web HTTPS 证书 |
| `web_key_file` | Web HTTPS 私钥 |
| `web_base_url` | Web 子路径，会作用于页面、`/api/`、`/captcha/`、静态资源和 session cookie |
| `web_close_on_not_found` | 未命中 Web 路径时尽量断开连接 |
| `head_custom_code` | 插入管理页面头部的自定义代码 |

修改 `web_username` / `web_password` 后重启 NPS 生效。正式环境不要使用示例密码。

`web_base_url` 会自动规范化，例如 `nps`、`/nps/` 都会变成 `/nps`。

## 登录保护

| 名称 | 说明 |
| --- | --- |
| `open_captcha` | 是否启用验证码 |
| `totp_secret` | TOTP 密钥 |
| `force_pow` | 是否对 session 登录强制要求 PoW |
| `pow_bits` | PoW 位数 |
| `login_ban_time` | 两次登录请求最小间隔 |
| `login_ip_ban_time` | IP 失败次数重置周期 |
| `login_user_ban_time` | 用户失败次数重置周期 |
| `login_max_fail_times` | 最大失败次数 |
| `login_max_body` | 登录请求体最大大小 |
| `login_max_skew` | 时间戳偏移容忍度 |
| `login_acl_mode` | 登录静态来源 ACL 模式 |
| `login_acl_rules` | 登录静态来源 ACL 规则 |

规则：

- `login_acl_mode=1` 表示白名单，`login_acl_mode=2` 表示黑名单。
- `login_acl_rules` 支持 IP、CIDR、IPv4 前缀通配符、`geoip:xx`、`geoip:private`。
- `POST /api/auth/session`、`POST /api/auth/token`、`POST /api/auth/register` 都会走登录来源 ACL。
- 启用 `totp_secret` 后，可单独提交 TOTP，也可把 6 位 TOTP 追加到密码末尾。
- 未启用 TOTP 时不允许纯空密码登录。
- `force_pow=false` 时，PoW 只在 `secure_mode=true` 且命中登录失败封禁时要求。

## 真实 IP 与前置代理

| 名称 | 说明 |
| --- | --- |
| `allow_x_real_ip` | 允许从受信代理头读取真实来源 IP |
| `trusted_proxy_ips` | 受信代理 IP，多个用逗号分隔 |
| `http_add_origin_header` | 业务 HTTP / HTTPS 转发时添加真实 IP 头 |
| `x_nps_http_only` | 前置代理与 NPS 之间的共享口令 |

当请求来自受信代理，或携带合法 `X-NPS-Http-Only` 口令时，认证链优先读取 `X-Forwarded-For` 的首个有效 IP，再回退 `X-Real-IP`。

`x_nps_http_only` 常用于 Nginx / Caddy 前置代理后，避免 NPS 重复做 HTTPS 跳转。

## Nginx 示例

```nginx
server {
    listen 443 ssl;
    server_name example.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://127.0.0.1:80;
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

## 独立前端

| 名称 | 说明 |
| --- | --- |
| `web_standalone_allowed_origins` / `standalone_allowed_origins` | 允许来源，逗号分隔，支持 `*` |
| `web_standalone_allow_credentials` / `standalone_allow_credentials` | 是否允许浏览器携带凭据 |
| `web_standalone_token_secret` / `standalone_token_secret` | standalone token 签名密钥 |
| `web_standalone_token_ttl_seconds` / `standalone_token_ttl_seconds` | token 有效期 |

这些配置只在前端和服务端分开部署时需要。
