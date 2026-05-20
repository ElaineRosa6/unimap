# 0002: Bridge API 独立鉴权边界

**日期:** 2026-05-09
**状态:** 已采纳
**来源:** `slice2-bridge-auth-contract.md`

## 背景

Chrome 扩展桥接 API（`/api/screenshot/bridge/*`）需要独立的身份验证，不能与 Web 管理 session 混用。

## 决策

1. **Bridge Token 独立命名** — 与 Web Admin Token、CLI/API Token 分域使用
2. **Bearer Token 校验** — 扩展任务获取和回调使用独立 Bearer token，不依赖 Web session
3. **HMAC-SHA256 签名** — callback 接口使用带时间戳和 nonce 的签名校验，防重放
4. **Loopback 检查** — 配对接口限制本地请求（X-Forwarded-For, X-Real-IP, RemoteAddr, Host）
5. **Token 生命周期** — TTL + 轮换 + 撤销，仅管理员或内部调度触发
6. **审计日志** — 所有 token 操作记录审计日志
7. **脱敏状态** — Bridge 状态接口返回脱敏信息，不返回完整 token
8. **plugin_mode 三态** — `off` / `auto` / `required`，auto 仅做能力发现，不隐式触发截图

## 后果

- Web session 存在不能绕过 Bridge API 鉴权
- CLI 不应持有 Bridge Token，只持有 CLI/API Token
- 扩展配对保留 loopback/origin 检查、短期 token、nonce
