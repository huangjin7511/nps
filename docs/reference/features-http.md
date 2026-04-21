# 站点与 HTTP

这一组页面专门说明域名转发里的站点能力，不重复写第一次创建规则的操作步骤。

如果你要先完成网站发布验证，先看 [域名转发](/guide/tunnels/domain-forwarding.md)。
如果你要处理证书、真实 IP 或前置反向代理，先看 [HTTPS 与反向代理](/guide/server/https-and-proxy.md)。

## 先按问题找

| 你要确认什么 | 建议页面 |
| --- | --- |
| 站点保护、自动证书、自动 HTTPS、TLS 直通或 TLS 终止 | [证书、TLS 与站点保护](/reference/features-http-tls.md) |
| Host 修改、自定义重定向、请求 Header、响应 Header、自动 CORS | [Header、重定向与 CORS](/reference/features-http-headers.md) |
| 泛域名、URL 路由、URL 重写、404 页面 | [URL 路由、重写与 404](/reference/features-http-routing.md) |

## 这一组页面不解决什么

- 客户端到服务端的连接协议、压缩和加密：看 [传输与连接](/reference/features-transport.md)
- TCP / UDP / 端口映射 / 端口复用：看 [代理、转发与路由](/reference/features-routing.md)
- ACL、带宽、流量和连接数限制：看 [访问控制与限制](/reference/features-access.md)

## 推荐阅读顺序

1. [证书、TLS 与站点保护](/reference/features-http-tls.md)
2. [Header、重定向与 CORS](/reference/features-http-headers.md)
3. [URL 路由、重写与 404](/reference/features-http-routing.md)
