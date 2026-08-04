# ADR：Extension 凭据向 CDP 的同源交接

- 状态：Accepted
- 日期：2026-08-02

## 背景

浏览器引擎可能只在用户日常 Chrome/Extension 会话中保持登录，而本地 CDP headless 没有等价会话。部分站点除 Cookie 外还依赖 localStorage/sessionStorage。

## 决策

Bridge 增加 `get_browser_credentials` 动作与类型化 `cookies`、`storage`、`final_url` 回调字段。扩展只在任务指定引擎的同源标签页读取浏览器凭据；服务端再次验证最终 URL 同源。Cookie 沿用受保护配置持久化，Web Storage 仅保存在进程内，并在 CDP 导航到同源页面后注入和刷新。

该能力不改变 SSRF 边界：Bridge 端点仍限 loopback，搜索 URL 仍为固定引擎白名单，CDP 全请求拦截与连接级 IP 固定继续生效。受控上游只接受 literal-loopback SOCKS5，并且只收到 UniMap 实时复核后的固定公网 IP:port；远程 Chrome、HTTP/外部上游代理不因此放行。

## 后果

- Censys/DayDayMap 等依赖 Web Storage 的会话可以复用到 CDP。
- 进程重启后 Web Storage 需从在线 Extension 再次交接；避免把站点存储内容写入磁盘或日志。
- 交接成功只证明凭据传递；挑战页会结构化为 `browser_challenge`，并在 `auto`/fallback 对单次任务切换到 Extension；验证码、网络超时或空资产仍不能误报为 CDP 采集通过。
- Bridge task DTO 的 `query` 和新增回调字段构成协议，服务端、扩展、API 文档与测试必须同步修改。
