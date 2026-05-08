# Extension 模式问题清单

> **创建日期：** 2026-05-09
> **分支：** `release/major-upgrade-vNEXT`
> **来源：** 用户反馈 + 代码深度分析
> **状态：** 待修复

---

## 问题总览

| # | 问题 | 严重级别 | 影响范围 |
|---|------|---------|---------|
| 1 | UQL 查询语法未翻译直接发送到搜索引擎 | 🔴 Critical | Extension + CDP 浏览器模式 |
| 2 | 查询进度一直显示 0%，完成后跳 100% | 🟠 High | WebSocket 查询 |
| 3 | 结构化数据无法返回前端 | 🔴 Critical | Extension collect 模式 |
| 4 | 搜索引擎登录状态同步不准确 | 🟠 High | UI 登录状态面板 |

---

## 问题 1：UQL 查询语法未翻译

### 现象

用户输入 UQL 语法（如 `country="CN" && port="80"`），在 Extension/CDP 浏览器模式下，查询语句被**原样**发送给搜索引擎，没有经过引擎适配器的 `Translate()` 方法翻译。

- **FOFA**：碰巧 UQL 语法与 FOFA 原生语法相似，简单查询可用
- **Hunter**：部分查询可用，复杂查询失败
- **ZoomEye / Quake**：查询语法完全不同，直接报错

### 根因

`internal/screenshot/manager.go:728` 的 `BuildSearchEngineURL()` 直接将原始 query 进行 base64 编码拼接到 URL：

```go
func (m *Manager) BuildSearchEngineURL(engine, query string) string {
    b64Query := base64.StdEncoding.EncodeToString([]byte(query))  // ← 原始 UQL，未翻译
    switch strings.ToLower(engine) {
    case "fofa":
        return fmt.Sprintf("https://fofa.info/result?qbase64=%s", encodedB64)
    // ...
    }
}
```

调用链：

```
handleWebSocketQuery
  → runBrowserQueryAsync
    → RunBrowserQueryAsync
      → browserRouter.OpenSearchEngineResult(ctx, engine, query)   // query 是原始 UQL
        → ExtensionProvider.OpenSearchEngineResult
          → mgr.BuildSearchEngineURL(engine, query)                // ← 直接编码 UQL
```

对比正确的 API 查询流程：

```
UQLParser.Parse(query) → AST → FofaAdapter.Translate(AST) → 翻译后的查询 → API 调用
```

### 修复方案

在 `BuildSearchEngineURL` 或其上游增加 UQL 翻译步骤：

1. 注入各引擎 adapter 的 `Translate` 方法引用
2. 将 UQL query 解析为 AST
3. 调用对应引擎的 `Translate(ast)` 获取引擎原生查询语法
4. 将翻译后的查询用于 URL 构建

### 涉及文件

- `internal/screenshot/manager.go` — `BuildSearchEngineURL()`
- `internal/screenshot/router.go` — `ExtensionProvider.OpenSearchEngineResult()` / `CollectSearchEngineResult()`
- `internal/core/unimap/parser.go` — UQLParser

---

## 问题 2：查询进度卡在 0%

### 现象

WebSocket 查询发起后，进度条始终显示 0%，直到查询完成才跳到 100%。浏览器 collect 操作可能耗时 30s+，期间用户无法感知进度。

### 根因

`web/websocket_handlers.go:225-354` 的异步 goroutine 中，从查询开始到完成**从未调用** `updateQueryProgress()`：

```go
go func() {
    queryCtx, queryCancel := context.WithTimeout(ctx, 60*time.Second)
    defer queryCancel()

    browserQueryCh := s.runBrowserQueryAsync(...)
    resp, queryErr := s.service.Query(queryCtx, req)
    // ... 执行完毕 ...

    st.Progress = 100  // ← 唯一一次赋值：直接跳到 100
    // ...
}()
```

`updateQueryProgress()` 函数（第 373-392 行）存在且功能正常，但**没有任何代码调用它**。

### 修复方案

1. 在 `BrowserQueryOutcome` 中增加进度回调参数
2. 在 `RunBrowserQueryAsync` 中每完成一个引擎的操作（open/collect/capture）后调用进度回调
3. 在 `handleWebSocketQuery` 的 goroutine 中将进度更新转发为 `updateQueryProgress()` 调用
4. 建议进度粒度：
   - 每个引擎 open 完成 → +5%
   - 每个引擎 collect 完成 → +10%
   - 每个引擎 capture 完成 → +10%
   - 主查询完成 → 根据引擎数分配

### 涉及文件

- `web/websocket_handlers.go` — `handleWebSocketQuery()` goroutine
- `internal/service/query_app_service.go` — `RunBrowserQueryAsync()`
- `web/query_handlers.go` — `runBrowserQueryAsync()`

---

## 问题 3：结构化数据无法返回

### 现象

使用 Extension 模式的 collect 操作时，`browserCollectedData` 始终为空数组，前端无法展示采集到的结构化资产数据。

### 根因（连锁反应）

这是一个**复合问题**，由多个因素导致：

**根因 A：UQL 未翻译（问题 1 的延伸）**

搜索引擎收到的是它不认识的 UQL 语法，返回错误页面或登录页面。扩展采集到的是错误页面的内容，结构化数据为空。

**根因 B：CDP 模式 collect 创建新浏览器实例**

`internal/screenshot/manager.go:927` 的 `CollectSearchEngineResult()`：

```go
func (m *Manager) CollectSearchEngineResult(ctx, engine, query, queryID string) ([]CollectResult, error) {
    allocCtx, allocCancel, err := m.newAllocator(ctx)  // ← 新浏览器实例
    // ... 没有设置 cookies，没有复用已有 CDP 会话
    chromedp.Run(browserCtx, chromedp.Navigate(searchURL))
    // ... 在新浏览器中采集，用户未登录，碰到登录墙
}
```

**根因 C：Bridge 超时阻塞无中间反馈**

`internal/screenshot/bridge_service.go:99` 的 `Submit()` 阻塞等待结果，默认超时 30s：

```go
func (s *BridgeService) Submit(ctx context.Context, task BridgeTask) (BridgeResult, error) {
    // ... 阻塞等待 30s
    select {
    case out := <-job.respCh:
        return out.result, out.err
    case <-workerCtx.Done():
        return BridgeResult{}, fmt.Errorf("%w: task timeout", ErrBridgeTimeout)  // ← 30s 超时
    }
}
```

对于 collect 操作（打开浏览器 → 导航 → 等待 SPA 渲染 → JS 提取 → 回传），30s 不够。

**根因 D：扩展端采集逻辑与后端不匹配**

后端期望 `structured_collected_data` 格式，但扩展端可能：
- 只返回了 `collected_data` 字符串（旧格式）
- 采集的选择器/字段映射与后端 `parseStructuredCollectedData()` 不一致

### 修复方案

1. **修复问题 1**（UQL 翻译）是最关键的修复
2. CDP 模式 collect 复用已有 CDP 会话或注入 cookies
3. 增加 collect 超时时间到 60s，或支持可配置
4. Bridge 中间状态反馈机制（见问题 2 的修复）
5. 统一扩展端和后端的数据格式协议

### 涉及文件

- `internal/screenshot/manager.go` — `CollectSearchEngineResult()`
- `internal/screenshot/bridge_service.go` — `Submit()` / `taskTimeout`
- `internal/screenshot/router.go` — `ExtensionProvider.CollectSearchEngineResult()`
- `web/screenshot_bridge_handlers.go` — result handler

---

## 问题 4：登录状态同步不准确

### 现象

- 扩展已连接但用户未在搜索引擎登录 → UI 显示"已登录"
- 用户实际已登录 → UI 显示"未登录"或"检测中"
- 四个引擎（FOFA / Hunter / ZoomEye / Quake）全部显示"检测中"状态卡住

### 根因

`web/cookie_handlers.go:418-437` 中 `handleCookieLoginStatus()`：

```go
} else if extPaired {
    // 扩展配对了 → 所有引擎直接报告已登录
    for _, engine := range engines {
        item := map[string]interface{}{
            "engine":    engine,
            "logged_in": true,         // ← 永远为 true！没有实际验证
            "reason":    "browser_session",
        }
    }
}
```

问题：
- 扩展配对（WebSocket 连接成功） ≠ 用户在搜索引擎上已登录
- 此分支下不打开任何页面做实际检测，直接假设已登录
- CDP 模式下（第 375-417 行）只报告 cookies 是否配置，也不做实际页面检测（"避免干扰用户浏览"的设计取舍）

### 修复方案

**方案 A（推荐）**：通过 Bridge 让扩展实际打开引擎搜索页面并检测登录状态
- 扩展向每个引擎的搜索结果页发送探测请求
- 检测页面是否包含登录墙关键词（"登录"、"请登录"、"Sign In"）
- 将实际检测结果回传给后端

**方案 B**：扩展配对后，通过 Bridge 获取当前页面信息
- 扩展上报当前活跃页面的 URL 和标题
- 后端判断用户是否在目标引擎的搜索结果页（非登录页面）

**方案 C**：CDP 模式下复用已有会话做实际检测
- 当前 CDP 模式创建新会话（`newAllocator`），不利用已有登录态
- 修改为复用已有 CDP 连接（如果存在）做登录状态检测

### 涉及文件

- `web/cookie_handlers.go` — `handleCookieLoginStatus()`
- `internal/screenshot/manager.go` — `CheckEngineLoginStatus()`
- Chrome Extension 端 — 需要新增登录状态检测逻辑

---

## 修复优先级

1. **P0 — 问题 1（UQL 翻译）**：所有浏览器模式功能的基础，不修复其他问题都无法解决
2. **P0 — 问题 3（结构化数据）**：核心功能失效，用户感知最强
3. **P1 — 问题 2（进度反馈）**：用户体验问题，但功能本身可用
4. **P1 — 问题 4（登录状态）**：UI 显示问题，影响用户判断

## 修复依赖关系

```
问题 1 (UQL 翻译)
    ↓
问题 3 (结构化数据) ← 问题 1 修复后，搜索引擎返回正确页面
    ↓               ← 还需要增加超时 + CDP 复用
问题 2 (进度反馈)    ← 可以在问题 1 之后独立修复
问题 4 (登录状态)    ← 需要扩展端配合，可独立修复
```
