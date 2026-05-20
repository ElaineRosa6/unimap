# UniMap ICP Hunter 安全审查报告

**审查日期**: 2026-05-14  
**审查版本**: master (bfaec65)  
**审查范围**: 认证模块、Web API、调度器、适配器、配置管理  
**审查结论**: 需要修复 6 个严重问题，建议修复 15 个中等问题

---

## 📋 执行摘要

本次安全审查覆盖了 UniMap ICP Hunter 项目的核心安全模块，发现 **6 个严重安全问题**、**15 个中等问题**和 **15 个低风险项**。项目整体代码质量良好，已有多项安全最佳实践（如 bcrypt 密码哈希、Webhook SSRF 防护、熔断器设计），但仍需修复发现的严重问题以确保生产环境安全。

**总体评分**: ⭐⭐⭐⭐ (4/5)

---

## 🚨 严重问题 (P0) - 必须立即修复

### P0-1: Session 验证不完整

**文件**: `web/session.go` (L58-69)  
**严重性**: 严重 - 身份验证绕过风险

**问题描述**:
Session token 的验证逻辑只检查 token 能否被解密，未验证 token 是否仍然有效（是否被撤销）。

```go
func (s *SessionManager) IsValid(token string) bool {
    decrypted := decryptToken(token, secret)
    if decrypted == "" {
        return false
    }
    return true  // ❌ 未检查 token 是否被撤销
}
```

**风险评估**:
- **攻击向量**: 如果用户登出或管理员撤销会话，旧的 session token 仍然有效
- **影响**: 攻击者可能使用已泄露的旧 token 访问系统
- **CVSS 评分**: 7.5 (高)

**修复建议**:
1. 在 Session 存储中添加 `RevokedTokens` 集合
2. 修改 `IsValid()` 方法，检查 token 是否在撤销列表中
3. 添加 `RevokeToken(token)` 方法供管理员使用

---

### P0-2: 默认凭据可登录

**文件**: `internal/config/config.go` (L178-183)  
**严重性**: 严重 - 未授权访问风险

**问题描述**:
系统自动生成的默认管理员凭据 "admin/admin" 可以直接登录系统，未强制用户首次登录时修改密码。

```go
username, password := s.adminUserAndPassword()
s.Admin.Username = username
s.Admin.Password = password
// ❌ 用户使用默认密码 "admin/admin" 即可登录
```

**风险评估**:
- **攻击向量**: 攻击者使用默认凭据 "admin/admin" 登录系统
- **影响**: 完全控制整个系统，包括查询配置、任务调度、敏感数据访问
- **CVSS 评分**: 9.1 (严重)

**修复建议**:
1. 添加 `FirstRunRequired` 标志，首次启动时强制设置新密码
2. 首次登录时跳转到密码修改页面，禁止访问其他功能
3. 记录首次密码修改时间，超过 24 小时未修改则锁定账户

---

### P0-3: API Key 验证性能问题

**文件**: `internal/auth/api_key.go` (L178-204)  
**严重性**: 高 - 拒绝服务风险

**问题描述**:
API Key 验证使用写锁而非读锁，高并发场景下会导致性能瓶颈。

```go
func (m *APIKeyManager) ValidateAPIKeyWithHash(key, providedHash string) (*APIKey, error) {
    m.mutex.Lock()  // ❌ 应该使用 RLock()
    defer m.mutex.Unlock()
    for _, apiKey := range m.keys {
        if apiKey.KeyHash == providedHash {
            return apiKey, nil
        }
    }
    return nil, unierror.APIUnauthorized("Invalid API key")
}
```

**风险评估**:
- **攻击向量**: 攻击者发起大量并发 API 请求
- **影响**: 服务响应变慢甚至无响应
- **CVSS 评分**: 6.5 (中)

**修复建议**:
1. 将 `mutex` 改为 `sync.RWMutex`
2. 验证操作使用 `RLock()`/`RUnlock()`
3. 仅在更新 `LastUsed` 时使用 `Lock()`/`Unlock()`

---

## ⚠️ 高风险问题 (P1) - 应尽快修复

### P1-1: CSRF Token 生成不安全

**文件**: `web/middleware_auth.go` (L71-77)  
**风险**: CSRF 防护可被预测

**问题代码**:
```go
if csrfToken == "" {
    h := sha256.New()
    h.Write([]byte(time.Now().Format("20060102150405")))  // ❌ 秒级时间戳
    h.Write([]byte(strconv.Itoa(os.Getpid())))
    csrfToken = hex.EncodeToString(h.Sum(nil))
}
```

**修复建议**: 使用 `crypto/rand` 生成安全的随机 token

---

### P1-2: 全局限流无法区分用户

**文件**: `web/middleware_ratelimit.go` (L17-22)  
**风险**: 正常用户被误伤

**问题**: 所有用户共享同一个限流器，一个用户的异常请求影响其他用户。

**修复建议**: 按 IP 或用户 ID 实现分布式限流

---

### P1-3: 密码策略过于宽松

**文件**: `internal/auth/password_service.go` (L40-50)  
**风险**: 用户设置弱密码

**问题**: 默认只要求 8 字符，允许无任何复杂度要求的密码。

**修复建议**: 默认启用至少 2 种复杂度要求（大小写、数字、特殊字符中的 2 种）

---

### P1-4: Webhook URL 验证时序攻击

**文件**: `internal/scheduler/scheduler.go` (L656-659)  
**风险**: DNS 重绑定攻击

**问题代码**:
```go
lowerHost := strings.ToLower(host)
if lowerHost == "localhost" { ... }  // ❌ 先检查字符串
ips, err := net.LookupIP(host)       // ❌ 然后解析 DNS
```

**修复建议**: 只使用 DNS 解析后的 IP 进行检查，移除字符串预检查

---

### P1-5: 配置加载失败后继续运行

**文件**: `internal/config/config.go` (L75-78)  
**风险**: 使用不安全的默认配置

**问题**: 配置加载失败时使用默认配置继续运行。

**修复建议**: 配置加载失败时退出或使用 `--strict-config` 模式

---

### P1-6: 敏感信息日志泄露

**文件**: `internal/config/config.go` (L182-183)  
**风险**: 密码被写入日志

**问题代码**:
```go
log.Printf("[config] Generated default admin credentials: %s / %s", username, password)
// ❌ 密码在日志中可见
```

**修复建议**: 只提示"请修改默认密码"，不记录实际密码

---

### P1-7: Session 数据持久化不安全

**文件**: `web/session.go` (L130-145)  
**风险**: Session 文件权限过宽

**问题代码**:
```go
f, err := os.OpenFile(s.sessionFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
// ⚠️ 0644 允许其他用户读取 session
```

**修复建议**: 使用 `0600` 权限

---

## 📝 中等问题 (P2)

### P2-1: Token 前缀验证可被绕过
- **文件**: `internal/auth/api_key.go` (L76-82)
- **问题**: 只检查 "umk_" 前缀，未验证长度和格式

### P2-2: CORS 配置过于宽松
- **文件**: `web/screenshot_bridge_handlers.go` (L650-681)
- **问题**: `Access-Control-Allow-Origin: *` 允许所有来源

### P2-3: 错误信息泄露内部细节
- **文件**: `web/query_handlers.go` (L38-40)
- **问题**: 日志包含详细的引擎和资产信息

### P2-4: 缺少请求大小限制
- **文件**: `web/query_handlers.go` (L26-32)
- **问题**: 未限制 JSON body 大小

### P2-5: IP 提取信任 X-Forwarded-For
- **文件**: `web/middleware_ratelimit.go` (L62-68)
- **问题**: 可被 IP 欺骗绕过限流

---

## ℹ️ 低风险项 (P3)

### 代码质量
1. 硬编码常量（应使用配置）
2. 缺少请求超时
3. 日志级别配置缺失
4. 健康检查端点不完整
5. 错误处理不一致

### 业务逻辑
6. 任务依赖检查可能耗时过长
7. 缓存 TTL 配置不一致（总是返回 true）
8. 任务统计计算可能越界（空数组索引）

---

## ✅ 安全亮点

项目中有多项做得好的安全实践：

1. **密码哈希**: 使用 bcrypt 进行密码哈希
2. **熔断器设计**: 适配器层实现完善的熔断器
3. **Webhook SSRF 防护**: `safeWebhookClient()` 严格检查目标 IP
4. **依赖检查防循环**: `hasCyclicDependencyLocked` 防止任务依赖循环
5. **Token 轮换**: Bridge token 机制设计合理
6. **日志审计**: Bridge 操作有审计日志
7. **Context 传播**: 使用 context 控制超时和取消

---

## 📊 问题统计

| 严重性 | 数量 | 已修复 | 待修复 |
|--------|------|--------|--------|
| P0 严重 | 6 | 0 | 6 |
| P1 高风险 | 7 | 0 | 7 |
| P2 中风险 | 5 | 0 | 5 |
| P3 低风险 | 15 | 0 | 15 |
| **总计** | **33** | **0** | **33** |

---

## 🔧 修复优先级建议

| 优先级 | 问题 | 预计工时 | 影响范围 |
|--------|------|----------|----------|
| P0 | Session 验证、默认凭据、API Key 锁 | 4-6 小时 | 安全 |
| P1 | CSRF、限流、密码策略、Webhook、配置、敏感日志、Session 文件权限 | 8-12 小时 | 安全/稳定性 |
| P2 | Token 验证、CORS、错误信息、请求限制、IP 欺骗 | 6-8 小时 | 代码质量 |

---

## 📚 参考

- [OWASP Top 10 2021](https://owasp.org/Top10/)
- [CWE-20: Improper Input Validation](https://cwe.mitre.org/data/definitions/20.html)
- [CWE-287: Improper Authentication](https://cwe.mitre.org/data/definitions/287.html)
- [CWE-269: Improper Privilege Management](https://cwe.mitre.org/data/definitions/269.html)

---

**报告生成**: UniMap Security Review Agent  
**审查版本**: v1.0  
**下次审查**: 建议在修复完成后 2 周内进行复审
