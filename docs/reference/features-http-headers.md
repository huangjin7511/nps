# Header、重定向与 CORS

本页说明域名转发里的 Header 改写、重定向和 CORS。

## 自动 CORS

自动 CORS 用于临时解决浏览器跨域问题。启用后，NPS 会在请求带 `Origin` 时补充 CORS 响应头。

规则：

- 请求没有 `Origin` 时不处理。
- 后端已返回 `Access-Control-Allow-Origin` 时不强制覆盖。
- 会补充 `Access-Control-Allow-Credentials: true`。
- 正式环境更建议由后端返回精确 CORS 头。

## Host 修改

当内网站点要求的 `Host` 与公网访问域名不同，可使用域名转发里的 Host 修改字段。不要优先用自定义 Header 改 `Host`。

## 自定义重定向

支持返回 `307` 临时重定向。

```text
https://xxx.com${request_uri}
```

常用占位符：

| 占位符 | 含义 |
| --- | --- |
| `${scheme}` | `http` 或 `https` |
| `${ssl}` | `on` 或 `off` |
| `${host}` | 不带端口的主机名 |
| `${http_host}` | 原始 `Host` |
| `${server_port}` | 当前入口端口 |
| `${remote_addr}` | 客户端地址和端口 |
| `${remote_ip}` | 客户端 IP |
| `${remote_port}` | 客户端端口 |
| `${request_uri}` | 路径和查询字符串 |
| `${uri}` | 请求路径 |
| `${args}` | 查询字符串 |
| `${scheme_host}` | 协议和主机 |

## 请求 Header

可新增、修改或删除转发给后端的请求头。

```text
X-Original-URL: ${scheme_host}${request_uri}
X-Client-IP: ${remote_ip}
X-Forwarded-Proto: ${scheme}
```

额外占位符：

| 占位符 | 含义 |
| --- | --- |
| `${http_upgrade}` | 原始 `Upgrade` |
| `${http_connection}` | 原始 `Connection` |
| `${http_range}` | 原始 `Range` |
| `${http_if_range}` | 原始 `If-Range` |
| `${unset}` | 删除该头 |

重定向占位符也可用于请求 Header。

## 响应 Header

可新增、修改或删除返回给访问者的响应头。

```text
Access-Control-Allow-Origin: ${origin}
Access-Control-Allow-Credentials: true
```

常用占位符：

| 占位符 | 含义 |
| --- | --- |
| `${status}` / `${status_code}` | 后端响应状态 |
| `${content_length}` | 响应体长度，未知为 `-1` |
| `${content_type}` | 响应 `Content-Type` |
| `${origin}` | 请求 `Origin` |
| `${user_agent}` | 请求 `User-Agent` |
| `${http_referer}` | 请求 `Referer` |
| `${date}` | 当前 HTTP Date |
| `${timestamp}` | 当前秒级时间戳 |
| `${timestamp_ms}` | 当前毫秒时间戳 |
| `${unset}` | 删除该头 |

请求相关占位符也可用于响应 Header，例如 `${scheme}`、`${host}`、`${remote_ip}`、`${request_uri}`。
