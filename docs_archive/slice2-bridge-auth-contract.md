# Bridge API 鉴权边界契约

> Slice 2 执行日期: 2026-05-13
> 状态: 已实施

## 1. 设计原则

Bridge API (`/api/screenshot/bridge/*`) 有**完全独立**的鉴权边界,与普通 Web 管理 token、Session Cookie、API Key 互不混用。

### 1.1 核心约束

1. **Bridge token 不复用 Web session** - 扩展不能因 Web 登录态而获得 Bridge 访问权限
2. **CLI Token 不复用 Bridge Token** - CLI 只向 Web API 证明用户身份,Bridge Token 只向 Web 证明扩展身份
3. **配对接口保留安全检查** - loopback/origin 检查、短期 token、nonce、签名校验
4. **Token 轮换受控** - 只允许管理员或内部调度触发,记录审计日志
5. **CLI plugin_mode=auto 只做能力发现** - 不隐式触发截图任务
6. **Bridge 状态接口返回脱敏信息** - 不返回完整 token

## 2. 鉴权层次

### 2.1 Layer 0: Loopback IP 限制 (所有 Bridge 路由)

**实现位置**: `web/screenshot_bridge_handlers.go:isLoopbackRequest()`

**检查项**:
- `X-Forwarded-For` / `X-Real-IP` 必须为空
- `RemoteAddr` 必须是 loopback IP (127.0.0.1, ::1, localhost)
- `Host` 必须是 localhost 或 loopback IP

**失败响应**: `403 Forbidden` - `forbidden_origin`

### 2.2 Layer 1: Bearer Token 验证 (需要认证的 Bridge API)

**实现位置**: `web/bridge_auth.go:validateBridgeAuthIfRequired()`

**适用路径**:
- `/api/screenshot/bridge/tasks/next` - 拉取任务
- `/api/screenshot/bridge/token/rotate` - 轮换 token
- `/api/screenshot/bridge/mock/result` - 模拟回调

**不适用路径** (只需 loopback 检查):
- `/api/screenshot/bridge/health` - 健康检查
- `/api/screenshot/bridge/status` - 状态查询
- `/api/screenshot/bridge/pair` - 配对获取 token

**验证流程**:
1. 从 `Authorization: Bearer <token>` 提取 token
2. 调用 `validateBridgeToken()` 验证 token 存在且未过期
3. 调用 `touchBridgeToken()` 更新最后使用时间
4. 记录审计日志

**失败响应**: `401 Unauthorized` - `missing_bearer_token` 或 `invalid_bridge_token`

### 2.3 Layer 2: 回调签名验证 (可选,默认启用)

**实现位置**: `web/screenshot_bridge_handlers.go:validateBridgeCallbackSignatureIfRequired()`

**检查项**:
- HMAC-SHA256 签名验证
- 时间戳偏差检查 (默认 300s)
- Nonce 防重放 (存储在 `CallbackNonces` map)

**配置开关**: `screenshot.extension.callback_signature_required`

## 3. Token 生命周期

### 3.1 配对 (Pairing)

**端点**: `POST /api/screenshot/bridge/pair`

**请求**:
```json
{
  "client_id": "extension-unique-id",
  "pair_code": "user-provided-code"
}
```

**响应**:
```json
{
  "success": true,
  "token": "base64-url-safe-24-byte-random",
  "expires_in": 600,
  "expire_at": 1715587200
}
```

**安全约束**:
- 仅允许 loopback 请求
- Token TTL 默认 600 秒 (可配置)
- 记录 `LastPairAt` 时间戳

### 3.2 Token 轮换 (Rotate)

**端点**: `POST /api/screenshot/bridge/token/rotate`

**认证**: 需要有效的 Bearer Token

**响应**:
```json
{
  "success": true,
  "token": "new-base64-url-safe-token",
  "expires_in": 600,
  "expire_at": 1715587200
}
```

**安全约束**:
- 旧 token 立即失效
- 清理关联的 nonce
- 记录审计日志: `bridge_audit: action=rotate token_prefix=xxx`

### 3.3 Token 撤销 (Revoke)

**内部方法**: `s.revokeBridgeToken(token)`

**用途**:
- Token 轮换时撤销旧 token
- 扩展主动断开时撤销
- 管理员手动撤销

**清理项**:
- 从 `Tokens` map 删除
- 从 `CallbackNonces` 清理相关 nonce
- 从 `LastSeen` map 删除

## 4. 审计日志格式

### 4.1 认证成功

```
bridge_auth: token validated (prefix=abc12345) for path=/api/screenshot/bridge/tasks/next from=127.0.0.1:54321
```

### 4.2 认证失败

```
bridge_auth: missing_bearer_token / invalid_bridge_token (记录到 LastErr)
```

### 4.3 配对事件

```
bridge_audit: action=pair token_prefix=xyz78901 detail=client_id=ext-123 at=1715587200
```

### 4.4 Token 轮换

```
bridge_audit: action=rotate token_prefix=abc12345 detail=old_token_revoked at=1715587200
```

### 4.5 回调签名验证

```
bridge_audit: action=callback_verify token_prefix=abc12345 detail=nonce=xxx,timestamp=1715587200 at=1715587200
```

## 5. 与 Web 认证的关系

### 5.1 公开路径 ( bypass adminAuthMiddleware)

```go
publicPrefixes := []string{
    "/api/screenshot/bridge/",  // Bridge 有自己的鉴权
}
```

**原因**: Bridge API 由 Chrome 扩展调用,不应依赖 Web session 或 Admin Token

### 5.2 内部调用链

```
Chrome Extension
  -> POST /api/screenshot/bridge/pair (loopback only)
  -> GET  /api/screenshot/bridge/tasks/next (Bearer Token)
  -> POST /api/screenshot/bridge/mock/result (Bearer Token + Signature)

Web UI (管理员)
  -> GET  /api/screenshot/bridge/health (Admin Token / Session)
  -> GET  /api/screenshot/bridge/status (Admin Token / Session, 返回脱敏状态)
```

### 5.3 脱敏规则

`/api/screenshot/bridge/status` 返回给 Web UI 的状态信息:
- ✅ `LastPairAt` - 最后配对时间
- ✅ `LastTaskPullAt` - 最后任务拉取时间
- ✅ `LastCallbackAt` - 最后回调时间
- ✅ `LastErr` - 最后错误
- ❌ `Tokens` - 不返回完整 token,只返回数量
- ❌ `CallbackNonces` - 不返回

## 6. 配置字段

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `screenshot.extension.enabled` | bool | `false` | 启用扩展模式 |
| `screenshot.extension.listen_addr` | string | `127.0.0.1:19451` | 监听地址 |
| `screenshot.extension.pairing_required` | bool | `true` | 需要配对 |
| `screenshot.extension.token_ttl_seconds` | int | `600` | Token TTL |
| `screenshot.extension.callback_signature_required` | bool | `false` | 回调签名 |
| `screenshot.extension.callback_signature_skew_seconds` | int | `300` | 时间偏差容忍 |
| `screenshot.extension.callback_nonce_ttl_seconds` | int | `600` | Nonce TTL |
| `screenshot.extension.fallback_to_cdp` | bool | `true` | 降级到 CDP |

## 7. 回滚方案

- 恢复旧的桥接鉴权路径 (loopback + bearer token 逻辑不变)
- 保留新增的审计日志 (不影响主线)
- 关闭 `callback_signature_required` 可跳过签名验证

## 8. 验收标准

- [x] Bridge API 路径在 `isPublicPath()` 中声明为公开 ( bypass Web auth)
- [x] `validateBridgeAuthIfRequired()` 独立验证 bearer token
- [x] 配对接口强制 loopback 检查
- [x] Token 轮换记录审计日志
- [x] 回调签名验证支持 HMAC-SHA256
- [x] Bridge status 接口返回脱敏状态 (不暴露 token)
- [x] `go build ./...` 通过
- [x] `go test -race ./web/...` 通过
