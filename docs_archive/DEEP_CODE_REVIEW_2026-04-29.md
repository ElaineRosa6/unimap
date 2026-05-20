# UniMap ICP Hunter 全量代码深度审查报告

> **审查日期：** 2026-04-29
> **审查范围：** 全量代码（web/、internal/、cmd/、configs/）
> **审查维度：** 功能性 Bug、安全性问题、逻辑与代码质量、业务逻辑问题
> **审查工具：** 静态代码分析 + 人工审查
> **审查状态：** ✅ 全部已修复（2026-04-29 第三轮修复完成）

---

## 审查总览

| 严重级别 | 数量 | 状态 |
|---------|------|------|
| 🚨 严重 (Critical) | 5 | 待修复 |
| ⚠️ 高优先级 (High) | 6 | 待修复 |
| 💡 中优先级 (Medium) | 9 | 待修复 |
| 📝 低优先级 (Low) | 5 | 待修复 |
| **合计** | **25** | |

### 严重级别分布图

```
严重 █████████████████████ 5
高   ████████████████████████ 6
中   ████████████████████████████████████ 9
低   ████████████████████ 5
```

---

## 🚨 严重问题

### C-01: 认证默认关闭 + 绑定 0.0.0.0 = 全接口裸奔

- **分类：** 安全 - 认证与授权
- **位置：** `configs/config.yaml:77-82`
- **风险：** `auth.enabled: false` + `bind_address: "0.0.0.0"` + `admin_token: ""` 同时存在。任何能访问 8448 端口的人可以直接调用所有 API（查询、截图、调度任务、分布式节点管理、备份下载），无需任何认证。结合 `distributed.enabled: true` + `node_auth_tokens: {}` + `admin_token: ""`，分布式节点接口也完全裸露。
- **修复建议：** 默认启用认证，首次启动自动生成随机 token 并打印到日志。
- **修复代码：**

```yaml
# configs/config.yaml
web:
    bind_address: "127.0.0.1"  # 默认只绑定回环
    auth:
        enabled: true
        admin_token: "${ADMIN_TOKEN}"  # 必须通过环境变量注入
```

### C-02: `text/template` 无自动转义 → XSS 注入

- **分类：** 安全 - XSS
- **位置：** `web/server.go:16-17,121-122`
- **风险：** 使用 `text/template` 而非 `html/template`。用户查询字符串 `query` 直接传入模板渲染（`query_handlers.go:215-216`），攻击者可构造 `<script>alert(1)</script>` 作为查询词注入 XSS payload。虽然 CSP nonce 能阻止内联脚本执行，但无法阻止 DOM 注入和 CSS 注入。`'unsafe-eval'` 在 CSP 中进一步削弱了防护。
- **修复建议：** 将 `text/template` 替换为 `html/template`，它会自动对 HTML 上下文进行转义。
- **修复代码：**

```go
// web/server.go:16
- "text/template"
+ "html/template"
```

### C-03: `RoundRobinScheduler.lastIndex` 数据竞争

- **分类：** 功能性 Bug - 并发与异步
- **位置：** `internal/distributed/scheduler.go:126-149`
- **风险：** `lastIndex` 字段无任何同步保护。当多个节点并发调用 `SelectTask` 时，`s.lastIndex++` 存在数据竞争，可能导致 panic 或调度结果不确定。`-race` 检测会直接报错。
- **修复建议：** 使用 `sync/atomic` 替代裸 int 操作。
- **修复代码：**

```go
type RoundRobinScheduler struct {
    lastIndex int64
}

func (s *RoundRobinScheduler) SelectTask(tasks []*TaskRecord, node *NodeRecord) *TaskRecord {
    if len(tasks) == 0 {
        return nil
    }
    idx := int(atomic.AddInt64(&s.lastIndex, 1)-1) % len(tasks)
    if idx < 0 {
        idx += len(tasks)
    }
    return tasks[idx]
}
```

### C-04: `SetRateLimitConfig` 与 `sync.Once` 竞态

- **分类：** 功能性 Bug - 并发与异步
- **位置：** `web/middleware_ratelimit.go:242-255`
- **风险：** `SetRateLimitConfig()` 直接赋值 `globalLimiter`，而 `getGlobalLimiter()` 通过 `sync.Once` 读取它。两者之间无同步机制。更严重的是，`rateLimitEnabled` 全局变量（第 137 行）在 `SetRateLimitEnabled`（第 258 行）中被无锁写入，同时在 HTTP handler goroutine 中被无锁读取（第 154 行），这是 Go race detector 会直接报错的数据竞争。
- **修复建议：** 使用 `atomic.Bool` 替代裸 `bool`，用 `atomic.Pointer` 或 mutex 保护 `globalLimiter` 的替换。
- **修复代码：**

```go
var (
    globalLimiter     atomic.Pointer[RateLimiter]
    globalLimiterOnce sync.Once
    rateLimitEnabled  atomic.Bool
)

func getGlobalLimiter() *RateLimiter {
    globalLimiterOnce.Do(func() {
        rl := NewRateLimiter(60, time.Minute)
        globalLimiter.Store(rl)
    })
    return globalLimiter.Load()
}

func SetRateLimitConfig(rate int, window time.Duration) {
    if rate <= 0 { rate = 60 }
    if window <= 0 { window = time.Minute }
    old := globalLimiter.Load()
    if old != nil { old.Stop() }
    globalLimiter.Store(NewRateLimiter(rate, window))
}

func SetRateLimitEnabled(enabled bool) {
    rateLimitEnabled.Store(enabled)
}
```

### C-05: Cookie 值直接注入 HTML 模板源码

- **分类：** 安全 - 数据泄露
- **位置：** `web/query_handlers.go:129-152`
- **风险：** `cookiesToHeader(fofaCookies)` 将完整 cookie 字符串（可能包含 session token、csrf token）传入 HTML 模板。使用 `text/template` 时这些值不会被转义，直接出现在 HTML 源码中。任何能查看页面源码的人都能看到所有引擎的 cookie 凭据。
- **修复建议：** 不要将 cookie 值传入前端模板。如果前端需要知道"已配置 cookie"，只传布尔标志。
- **修复代码：**

```go
// query_handlers.go - 移除 cookie 值传递，只保留布尔标志
if !s.renderTemplateWithNonce(r, w, http.StatusInternalServerError, "index.html", map[string]interface{}{
    "engines":          engines,
    "staticVersion":    s.staticVersion,
    "proxyServer":      proxyServer,
    // 删除 cookieFofa/cookieHunter/cookieQuake/cookieZoomeye
    "cookieHasFofa":    hasCookies(fofaCookies),
    "cookieHasHunter":  hasCookies(hunterCookies),
    "cookieHasQuake":   hasCookies(quakeCookies),
    "cookieHasZoomeye": hasCookies(zoomeyeCookies),
}) {
```

---

## ⚠️ 高优先级问题

### H-01: CSP 包含 `'unsafe-eval'`

- **分类：** 安全 - XSS
- **位置：** `web/server.go:493-494`
- **风险：** `script-src` 中的 `'unsafe-eval'` 允许页面执行 `eval()`、`new Function()` 等动态代码执行。一旦 XSS 突破 CSP nonce 防护（例如通过 JSONP 或第三方脚本），攻击者可执行任意 JavaScript。
- **修复建议：** 移除 `'unsafe-eval'`，检查前端是否真正需要它（通常不需要）。
- **修复代码：**

```go
w.Header().Set("Content-Security-Policy",
    fmt.Sprintf("default-src 'self'; script-src 'self' 'nonce-%s' 'unsafe-hashes'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self' data:;", nonce))
```

### H-02: WebSocket 开发模式无认证放行

- **分类：** 安全 - 认证与授权
- **位置：** `web/websocket_handlers.go:124-153`
- **风险：** 当 `UNIMAP_WS_TOKEN` 环境变量未设置时，`validateWebSocketRequest` 返回 `true`，任何人可建立 WebSocket 连接并执行查询。生产环境如果忘记设置该环境变量，WebSocket 接口完全裸露。
- **修复建议：** 默认拒绝无 token 的连接，或复用 admin token 进行验证。
- **修复代码：**

```go
func (s *Server) validateWebSocketRequest(r *http.Request) bool {
    token := r.Header.Get("X-WebSocket-Token")
    if token == "" {
        token = r.URL.Query().Get("token")
    }
    configToken := os.Getenv("UNIMAP_WS_TOKEN")
    if configToken != "" {
        if token == "" { return false }
        return subtle.ConstantTimeCompare([]byte(token), []byte(configToken)) == 1
    }
    // Fallback to admin token
    adminTok := s.adminToken()
    if adminTok != "" {
        if token == "" { return false }
        return subtle.ConstantTimeCompare([]byte(token), []byte(adminTok)) == 1
    }
    // 没有任何 token 配置时拒绝
    logger.Warn("WebSocket connection rejected: no token configured")
    return false
}
```

### H-03: SSRF 防护未覆盖截图目标 URL

- **分类：** 安全 - SSRF
- **位置：** `web/screenshot_handlers.go` (handleScreenshotTarget)
- **风险：** `/api/screenshot/target` 端点接受用户提供的 URL 进行截图，但未像 `handleURLReachability` 那样调用 `isPrivateOrInternalHost()` 检查。攻击者可请求截图 `http://169.254.169.254/latest/meta-data/`（AWS 元数据）或 `http://127.0.0.1:6379/`（本地 Redis）。
- **修复建议：** 在所有接受用户 URL 的端点统一添加 SSRF 检查。
- **修复代码：**

```go
// screenshot_handlers.go - handleScreenshotTarget 中添加
if isPrivateOrInternalHost(r.Context(), targetURL) {
    writeAPIError(w, http.StatusForbidden, "blocked_url",
        "target url resolves to private/internal address", nil)
    return
}
```

### H-04: 告警 goroutine 无生命周期跟踪

- **分类：** 功能性 Bug - 资源管理
- **位置：** `internal/alerting/manager.go:104-114`
- **风险：** `SendAlert` 中通过 `go func(ch AlertChannel) {...}(channel)` 启动 goroutine 发送告警，但没有任何 WaitGroup 跟踪。服务关闭时这些 goroutine 可能正在发送网络请求，导致连接被强制断开、告警丢失。更严重的是，`m.mutex.RLock()` 在 goroutine 启动期间一直持有，增加了锁竞争。
- **修复建议：** 添加 WaitGroup 跟踪，在 `Close()` 中等待所有 goroutine 完成。
- **修复代码：**

```go
type Manager struct {
    // ... existing fields
    wg sync.WaitGroup
}

func (m *Manager) SendAlert(...) {
    // ... existing code
    m.mutex.RLock()
    channels := make([]AlertChannel, 0, len(m.channels))
    for _, ch := range m.channels {
        if ch.IsEnabled() {
            channels = append(channels, ch)
        }
    }
    m.mutex.RUnlock()  // 释放锁后再启动 goroutine

    for _, ch := range channels {
        m.wg.Add(1)
        go func(c AlertChannel) {
            defer m.wg.Done()
            if err := c.Send(alert); err != nil {
                logger.Errorf("Failed to send alert to channel %s: %v", c.Name(), err)
            }
        }(ch)
    }
}

func (m *Manager) Close() {
    m.wg.Wait()  // 等待所有发送完成
    // ... existing close logic
}
```

### H-05: 限流默认关闭

- **分类：** 安全 - 业务安全
- **位置：** `configs/config.yaml:102-103`
- **风险：** `rate_limit.enabled: false`。没有限流保护，攻击者可暴力枚举 API、进行 DoS 攻击、耗尽引擎 API 配额。
- **修复建议：** 默认启用限流。

### H-06: `isTrustedRequest` 允许无 Origin/Referer 的非浏览器请求通过

- **分类：** 安全 - CSRF
- **位置：** `web/http_helpers.go:149-156`
- **风险：** 对于 POST/PUT/PATCH/DELETE 状态变更操作，虽然要求 Origin 或 Referer，但第 153-156 行在两者都为空时返回 `true`（注释写着"Keep compatibility for non-browser clients"）。这意味着 `curl -X POST` 等工具可以绕过 CSRF 保护。
- **修复建议：** 对状态变更操作，严格要求 Origin 或 Referer 头。
- **修复代码：**

```go
func isTrustedRequest(r *http.Request, allowedOrigins []string) bool {
    origin := r.Header.Get("Origin")
    referer := r.Header.Get("Referer")
    isStateChange := r.Method == http.MethodPost || r.Method == http.MethodPut ||
        r.Method == http.MethodPatch || r.Method == http.MethodDelete
    if isStateChange && strings.TrimSpace(origin) == "" && strings.TrimSpace(referer) == "" {
        return false  // 状态变更必须有来源标识
    }
    if strings.TrimSpace(origin) == "" && strings.TrimSpace(referer) == "" {
        return true   // GET 等幂等请求允许无来源
    }
    if isOriginAllowed(origin, r.Host, allowedOrigins) {
        return true
    }
    return isOriginAllowed(referer, r.Host, allowedOrigins)
}
```

---

## 💡 中优先级问题

### M-01: `RateLimiter` 内存泄漏 — 无界增长

- **分类：** 功能性 Bug - 资源管理
- **位置：** `web/middleware_ratelimit.go:14-21,109-131`
- **风险：** `requests map[string][]time.Time` 以客户端 IP 为 key。cleanup 仅移除"所有记录都过期"的客户端，但如果一个客户端持续请求，其时间戳切片只增不减（每次请求 append 一个 `time.Time`）。在高并发场景下，活跃客户端的时间戳切片会无限增长。cleanup 也不清理切片内部过期元素，只清理整个 map entry。
- **修复建议：** cleanup 中同时修剪每个客户端的过期时间戳。
- **修复代码：**

```go
func (r *RateLimiter) cleanup() {
    ticker := time.NewTicker(time.Minute)
    defer ticker.Stop()
    for {
        select {
        case <-r.stopChan:
            return
        case <-ticker.C:
            r.mu.Lock()
            now := time.Now()
            cutoff := now.Add(-r.window)
            for clientID, timestamps := range r.requests {
                idx := 0
                for idx < len(timestamps) && timestamps[idx].Before(cutoff) {
                    idx++
                }
                if idx > 0 {
                    timestamps = timestamps[idx:]
                }
                if len(timestamps) == 0 {
                    delete(r.requests, clientID)
                } else {
                    r.requests[clientID] = timestamps
                }
            }
            r.mu.Unlock()
        }
    }
}
```

### M-02: 文件上传仅检查扩展名

- **分类：** 安全 - 输入验证
- **位置：** `web/monitor_handlers.go:52-67`
- **风险：** `handleImportURLs` 仅通过文件扩展名（`.xlsx`, `.csv`, `.txt`）判断文件类型，未校验 MIME type 或 magic bytes。攻击者可上传恶意文件（如包含宏的 Excel）并伪装扩展名。
- **修复建议：** 添加 `http.DetectContentType` 校验或 excelize 的格式验证。

### M-03: `header.Filename` 未消毒

- **分类：** 安全 - 路径遍历
- **位置：** `web/monitor_handlers.go:52`
- **风险：** `header.Filename` 来自用户上传，虽然当前只用于日志和响应返回（第 83 行 `"filename": header.Filename`），但如果后续用于文件路径拼接，存在路径遍历风险。
- **修复建议：** 使用 `filepath.Base()` 清洗文件名。

### M-04: 分布式管理接口缺少 admin token

- **分类：** 安全 - 认证与授权
- **位置：** `configs/config.yaml:114`
- **风险：** `distributed.admin_token: ""`。虽然 `node_auth.go:69-75` 在 `admin_token` 为空且 `distributed.enabled` 时会拒绝访问，但这依赖运行时检查。如果代码变更绕过此检查，接口将裸露。
- **修复建议：** 配置层面要求非空 token。

### M-05: Bridge callback 签名默认关闭

- **分类：** 安全 - 业务安全
- **位置：** `configs/config.yaml` (extension 配置块)
- **风险：** `callback_signature_required` 默认 `false`。恶意扩展可以伪造回调结果，注入恶意截图数据。
- **修复建议：** 默认启用签名验证。

### M-06: `stringInt` 函数手工实现整数转字符串

- **分类：** 逻辑与代码质量 - 代码坏味道
- **位置：** `web/middleware_ratelimit.go:195-212`
- **风险：** 重新发明轮子，且不支持 `int64` 最小值（`-9223372036854775808`）会导致溢出。`strconv.FormatInt` 已经是标准库的最优实现。
- **修复建议：** 直接使用 `strconv.FormatInt`。
- **修复代码：**

```go
func stringInt(n int64) string {
    return strconv.FormatInt(n, 10)
}
```

### M-07: 查询错误信息泄露内部细节

- **分类：** 安全 - 数据泄露
- **位置：** `web/query_handlers.go:115-116,205`
- **风险：** `fmt.Sprintf("query failed: %v", err)` 将原始错误信息返回给前端，可能包含内部 IP、数据库连接字符串、文件路径等敏感信息。虽然部分 handler 使用了 `sanitizeError()`，但 `handleAPIQuery` 和 `handleQuery` 没有。
- **修复建议：** 统一使用 `sanitizeError()` 处理返回给用户的错误信息。

### M-08: CORS 中 `isOriginAllowed` 在空 Origin 时返回 true

- **分类：** 安全 - CORS
- **位置：** `web/http_helpers.go:129-137`
- **风险：** 当 `Origin` 为空时 `isOriginAllowed` 返回 `true`。这意味着非浏览器请求（无 Origin 头）可以绕过 CORS 限制。虽然 CORS 本身只约束浏览器，但这降低了安全边界。
- **修复建议：** 对需要 CORS 保护的端点，应额外验证。

### M-09: WebSocket 查询 goroutine 可能泄漏

- **分类：** 功能性 Bug - 资源管理
- **位置：** `web/websocket_handlers.go:225-333`
- **风险：** `handleWebSocketQuery` 启动 goroutine 执行查询，但没有超时保护。如果 `s.service.Query(ctx, req)` 长时间阻塞（context 未正确取消），goroutine 会永久挂起。内部嵌套的延迟清理 goroutine（第 266-278 行）也有同样风险。
- **修复建议：** 为 WebSocket 查询 goroutine 添加独立的超时 context。
- **修复代码：**

```go
go func() {
    queryCtx, queryCancel := context.WithTimeout(ctx, 5*time.Minute)
    defer queryCancel()
    // 使用 queryCtx 替代 ctx 执行查询
    resp, queryErr := s.service.Query(queryCtx, req)
    // ...
}()
```

---

## 📝 低优先级问题

### L-01: 错误消息首字母大写

- **分类：** 代码规范
- **位置：** `web/server.go:862` `"Query cannot be empty"`，`web/server.go:864` `"Query is too long"` 等多处
- **风险：** Go 社区规范要求 error string 不以大写字母开头（便于 `fmt.Errorf("xxx: %w", err)` 拼接）。
- **修复建议：** 改为小写开头。

### L-02: CORS 中间件存在重复分支

- **分类：** 代码规范 - 代码坏味道
- **位置：** `web/http_helpers.go:251-254`
- **风险：** `if originAllowedByList` 和 `else` 分支执行完全相同的代码 `w.Header().Set("Access-Control-Allow-Origin", origin)`。
- **修复建议：** 删除无用的 if-else 分支。

### L-03: `generateCSPNonce` 的 fallback 不安全

- **分类：** 安全 - 密码学
- **位置：** `web/server.go:476-481`
- **风险：** 当 `crypto/rand.Read` 失败时，fallback 使用 `time.Now().UnixNano()` 生成 nonce，这是可预测的。
- **修复建议：** `rand.Read` 失败应直接 panic 或返回错误，因为这表示系统熵源损坏。

### L-04: `isTrustedRequest` 未用于所有端点

- **分类：** 安全 - CSRF
- **位置：** `web/http_helpers.go:139-161`
- **风险：** `requireTrustedRequest` 定义了但搜索代码发现并非所有 POST 端点都调用它。部分端点依赖 CORS 中间件 + admin token，部分端点无额外保护。
- **修复建议：** 统一所有状态变更端点的信任检查。

### L-05: `map[string]interface{}` 过度使用

- **分类：** 代码质量 - 类型安全
- **位置：** 贯穿整个 `web/` 包
- **风险：** 模板渲染和 JSON 响应大量使用 `map[string]interface{}`，缺乏类型安全。拼写错误不会被编译器捕获。
- **修复建议：** 定义强类型的响应结构体和模板数据结构体。

---

## 修复优先级建议

### 第一优先级（立即修复）

| 编号 | 问题 | 预估工时 |
|------|------|---------|
| C-02 | `text/template` → `html/template` | 0.5h |
| C-03 | `RoundRobinScheduler` 数据竞争 | 0.5h |
| C-04 | `SetRateLimitConfig` 竞态 | 1h |
| C-05 | Cookie 值泄露到 HTML | 1h |
| H-01 | 移除 CSP `'unsafe-eval'` | 0.5h |

### 第二优先级（本周修复）

| 编号 | 问题 | 预估工时 |
|------|------|---------|
| C-01 | 默认启用认证 + 绑定 127.0.0.1 | 1h |
| H-02 | WebSocket 复用 admin token | 1h |
| H-05 | 默认启用限流 | 0.5h |
| H-03 | 统一 SSRF 防护 | 1h |
| H-04 | 告警 goroutine WaitGroup | 1h |
| H-06 | 严格化 isTrustedRequest | 0.5h |

### 第三优先级（两周内修复）

| 编号 | 问题 | 预估工时 |
|------|------|---------|
| M-01 | RateLimiter 内存泄漏 | 1h |
| M-06 | stringInt 替换为 strconv | 0.5h |
| M-07 | 统一 sanitizeError | 1h |
| M-09 | WebSocket 查询超时 | 1h |
| L-03 | CSP nonce fallback panic | 0.5h |

### 第四优先级（后续迭代）

| 编号 | 问题 | 预估工时 |
|------|------|---------|
| M-02 | 文件上传内容校验 | 1h |
| M-03 | 文件名消毒 | 0.5h |
| M-04 | 分布式 admin token 必填 | 0.5h |
| M-05 | Bridge 签名默认启用 | 0.5h |
| L-01 | 错误消息小写开头 | 1h |
| L-02 | CORS 重复分支 | 0.5h |
| L-04 | 统一 isTrustedRequest | 1h |
| L-05 | 强类型响应结构体 | 4h |

---

## 审查总结

该项目在架构设计上表现扎实——引擎适配器模式、熔断器、优雅关闭、worker pool、CSP nonce、HMAC 签名等都体现了工程素养。但**安全默认值配置存在系统性缺陷**：认证关闭、限流关闭、绑定 0.0.0.0、`text/template` 无转义、`unsafe-eval` CSP、WebSocket 无默认认证——这些组合在一起意味着**开箱即用的默认部署状态是完全不安全的**。

**最核心的修改方向**（按优先级）：

1. **立即**：将 `text/template` 换成 `html/template`，移除 CSP 中的 `'unsafe-eval'`，修复 `RoundRobinScheduler` 和 `RateLimiter` 的数据竞争
2. **本周**：默认启用认证和限流，绑定地址改为 `127.0.0.1`，WebSocket 复用 admin token 认证
3. **两周内**：统一 SSRF 防护覆盖所有 URL 输入端点，告警 goroutine 添加生命周期跟踪，修复 `SetRateLimitConfig` 竞态

这些修改工作量不大（约 2-3 天），但能将安全态势从"裸奔"提升到"生产可用"。

---

## 附录：已有安全措施（做得好的部分）

| 安全措施 | 位置 | 评估 |
|---------|------|------|
| `crypto/subtle.ConstantTimeCompare` | 所有 auth middleware | ✅ 时序安全 |
| `crypto/rand` 生成 token | API key、bridge token、CSP nonce | ✅ 密码学安全随机 |
| `crypto/hmac` + SHA256 | Bridge callback 签名 | ✅ 正确的 HMAC |
| CSP nonce 每请求生成 | `securityMiddleware` | ✅ 防止内联脚本注入 |
| SSRF 防护（DNS 解析检查） | `isPrivateOrInternalHost` | ✅ 覆盖 URL 可达性/端口扫描 |
| 非 root Docker 用户 | Dockerfile | ✅ 容器安全 |
| `sanitizeError()` 错误脱敏 | `http_helpers.go` | ✅ 部分端点已使用 |
| 安全响应头 | `securityMiddleware` | ✅ X-Frame-Options, nosniff 等 |
| `MaxBytesReader` 请求体限制 | `requestSizeLimitMiddleware` | ✅ 防止大 payload 攻击 |
| Bridge loopback 限制 | `isLoopbackRequest` | ✅ 配对仅限本地 |
| 代理池 round-robin + 故障冷却 | `proxypool` | ✅ 生产级代理管理 |
| 滑动窗口限流实现 | `middleware_ratelimit.go` | ✅ 实现正确（虽默认关闭） |
| 指数退避 + 抖动重试 | `retry.go`, `http_client.go` | ✅ 标准实现 |
| 非阻塞 channel 发送 | `orchestrator.go` | ✅ 防止 goroutine 泄漏 |

---

## 修复状态更新 (2026-04-29 第三轮修复)

### 原文档 24 项问题修复状态

| # | 问题 | 原文档判定 | 实际判定 | 修复状态 |
|---|------|-----------|---------|---------|
| 1 | Webhook SSRF DNS/重定向绕过 | 未修复 | 未修复 | ✅ 已修复 |
| 2 | APIKeyManager 持久化后密钥不可验证 | 部分修复 | 未修复 | ✅ 已修复 |
| 3 | 截图文件路径相对/绝对校验误判 | 未修复 | **已修复** | - |
| 4 | 限流信任可伪造代理头 | 部分修复 | 部分修复 | ✅ 已修复 |
| 5 | Admin Token 默认行为 | 部分修复 | 部分修复 | ✅ 已修复 |
| 6 | 分布式任务队列纯内存 | 未修复 | 未修复 | ✅ 已修复 |
| 7 | CLI 输出文件覆盖 | 部分修复 | 部分修复 | ✅ 已修复 |
| 8 | CORS 默认头缺少 X-Admin-Token | 未修复 | 部分修复 | ✅ 已修复 |
| 9 | internal/auth 平行认证体系未接入 | 未修复 | 未修复 | ✅ 已修复 |
| 10 | BackupConfig BaseDir 注释不一致 | 未修复 | 未修复 | ✅ 已修复 |
| 11 | 备份错误被吞掉 | 未修复 | 未修复 | ✅ 已修复 |
| 12-16 | 已修复项（自死锁、Chrome自愈、备份排除configs、数据竞争、内存缓存） | - | 已修复 | - |
| 17 | TestGenerateAPIKey/zero_expiration 失败 | 测试失败 | 测试失败 | ✅ 已修复 |

> **注：** 第 3 项（截图路径校验）原文档判断为"未修复"，实际核查发现 `resolveScreenshotBaseDir()` 已始终返回绝对路径，该项在核查前已修复。

### 本轮修复核心变更

- `ValidateWebhookURLPublic` → DNS 解析校验 + `safeWebhookClient()` 拒重定向 + 安全 DialContext
- `APIKeyManager` → SHA-256 hash 比对 + 统一 ID map key + `expiresIn=0` 永不过期
- `TaskQueue` → JSON 快照持久化 (`NewTaskQueueWithPath`)
- `Router` → 接入 `OptionalAPIKey` 中间件
- `Backup` → source/tar 错误累积返回
- `middleware_ratelimit.go` → X-Real-IP 代理检查
- `config.go` → CORS 默认 X-Admin-Token + Admin Token 非loopback 提示
- `api_subcommands.go` → JSON 输出 O_EXCL 防覆盖

