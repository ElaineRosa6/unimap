# 巡检功能增强实施计划（v2 修正版）

> 基于 urlive.py 参考分析 + 代码逐行验证的误报诊断  
> 状态：✅ 全部完成（Phase 0-4，2026-06-25）  
> 页面命名：Web 导航"监控"→"巡检"

---

## 1. 误报根因诊断（代码验证版）

### 1.1 执行路径还原

`baidu.com` Balanced + Relaxed 模式，基线后立即检测即报"变动"。完整链路：

```
CheckTampering()
  → ComputePageHash()                          // 计算 7 个分段 hash + SimpleMD5Hash
  → detectMaliciousContent()                   // 对百度首页返回空
  → evaluateTamperChanges()
    ├─ simpleMD5Changed := SimpleMD5Hash 变化?  // ① 独立判定，不经过分段策略
    ├─ findChangedSegments()                    // ② 找出 hash 不同的分段
    │   └─ 7 个分段逐个 SHA-256 比较
    └─ if simpleMD5Changed || isMeaningfulTamper  // ③ OR 逻辑
         → result.Tampered = true               // ← ① 单方面可触发
```

### 1.2 三条真实根因链路

#### 根因 A：SimpleMD5Hash 绕过分段策略（P0）

```go
// detector_check.go:156-166
simpleMD5Changed := baseline.SimpleMD5Hash != "" && currentHash.SimpleMD5Hash != "" &&
    baseline.SimpleMD5Hash != currentHash.SimpleMD5Hash

// ...

if simpleMD5Changed || d.isMeaningfulTamper(changes) {  // ← OR
    result.Tampered = true
    result.Status = "tampered"
}
```

`computeSimpleMD5Hash()` 对 `<head>` + `<body>` 原始 HTML 做 MD5，**完全不受 `relaxedVolatileSegments` 影响**。任何 1 字节差异（时间戳、随机 ID、广告脚本、动态 `<style>`）都会导致 MD5 变化，直接 `Tampered=true`。

**这意味着**：即使 `isMeaningfulTamper` 改造得再完美，只要 `simpleMD5Changed` 一票还在，relaxed 模式就不可能消除误报。

#### 根因 B：分段 hasher 内部动态数据未归一化

`cleanHTML()` 只在 `computeElementHash()`（元素内容）和 `full_content`（comprehensive 模式）中调用。**以下分段 hasher 完全不经过 cleanHTML**：

| Hasher | 拼接的内容 | 动态噪声来源 |
|--------|-----------|-------------|
| `computeJSFileHash()` | `src + integrity + crossorigin + referrerpolicy` | 版本化文件名 `app.abc123.js`、cache-busting 参数 `?v=12345` |
| `computeScriptHash()` | `src + integrity + async + defer + content` | 内联分析脚本（百度统计 `_hmt.push`）、SSR 水合数据 |
| `computeStyleHash()` | `<style>` 文本 + `<link>` href | 动态样式、版本化 CSS 文件名 |
| `computeMetaHash()` | `name + property + content` | 动态 `<meta>` 如 `csrf-token` 的 content |
| `computeLinkHash()` | `href + text` | 链接文本的细微变化 |

**结论**：仅增强 `cleanHTML()` 无法修复这些分段 → 需在各自 hasher 内部做归一化。

#### 根因 C：状态语义不一致

```go
// detector_check.go:179-183
if len(changes) > 0 {         // isMeaningfulTamper 返回 false 才走到这里
    result.Status = "changed"
    result.Tampered = true    // ← 矛盾：不 meaningful 但 Tampered=true?
}
return "normal_dynamic"       // ← checkType 说 normal，result 却说 Tampered
```

调用方拿到的 `TamperCheckResult.Tampered = true`，但 `Status = "changed"`、`checkType = "normal_dynamic"`。语义混乱。

### 1.3 排除的假根因

| 误判 | 实际情况 |
|------|---------|
| ~~forms token value 变化~~ | `computeFormHash()` 只记录 `field:name:type`，不记录 `value`。token 值变化不影响 forms hash |
| ~~cleanHTML 不够强~~ | cleanHTML 只影响 element content 和 full_content，与 js_files/scripts/forms 的 hash 计算无关 |

---

## 2. 修正版 Phase 0：误报修复

> **不动**：API 签名、存储格式、strict 模式行为  
> **改动范围**：`detector_check.go` + `detector_hash.go` + 测试

### Step 0.1：SimpleMD5Hash 降级为辅助信号

**当前**：
```go
if simpleMD5Changed || d.isMeaningfulTamper(changes) {
    result.Tampered = true   // MD5 一票否决
```

**修正**：
```go
switch d.detectionMode {
case DetectionModeStrict:
    // strict: MD5 变化独立触发
    if simpleMD5Changed || d.isMeaningfulTamper(changes) {
        result.Tampered = true
        result.Status = "tampered"
        return "tampered"
    }
default:
    // relaxed/balanced/security/precise: MD5 变化仅作为辅助信息
    if d.isMeaningfulTamper(changes) {
        result.Tampered = true
        result.Status = "tampered"
        return "tampered"
    }
    // MD5 变了但分段无意义变化 → 归入动态波动
}
```

**效果**：relaxed 模式下切断最大误报源。SimpleMD5Hash 的值仍会记录在 `TamperCheckResult` 中供前端展示对比，但不独立触发告警。

### Step 0.2：修正状态语义

定义明确的三态：

```go
// 三态语义：
//   "normal"         — 无任何变化 (Tampered=false)
//   "normal_dynamic" — 有变化但属动态波动 (Tampered=false)
//   "tampered"       — 有意义的变化 (Tampered=true)

// 各分支保持一致：
if !simpleMD5Changed && len(changes) == 0 {
    result.Tampered = false
    result.Status = "normal"
    return "normal"
}
// ... isMeaningfulTamper → tampered ...
// 兜底：
result.Tampered = false          // ← 修正：不 meaningful 就不 tampered
result.Status = "normal_dynamic" // ← 与 checkType 对齐
return "normal_dynamic"
```

**同步修改**：`TamperAppService.Check()` 的 summary 统计需要区分：
- `tampered_count` — 只统计 `"tampered"`
- `normal_dynamic_count` — 单独统计（新增），不计入篡改

### Step 0.3：分段 hasher 内部归一化

**不做**：扩 cleanHTML 去覆盖它本来就碰不到的 hasher。

**做**：在每个波动分段内部，对拼接前的原始字段做归一化。

#### 0.3a `computeJSFileHash` — 版本化文件名 + cache 参数

```go
// detector_hash.go 新增包级正则
var (
    reVersionedFile = regexp.MustCompile(`[a-f0-9]{8,}\.(js|css)$`)
    reCacheBust     = regexp.MustCompile(`[?&](?:v|ver|version|t|ts|_|cb|nocache)=\d+`)
)

func normalizeAssetURL(raw string) string {
    raw = reVersionedFile.ReplaceAllString(raw, "HASH.$1")
    raw = reCacheBust.ReplaceAllString(raw, "")
    return raw
}

// computeJSFileHash 中：
src = normalizeAssetURL(src)
jsFiles = append(jsFiles, strings.Join([]string{src, integrity, crossorigin, referrerpolicy}, ":"))
```

#### 0.3b `computeScriptHash` — SSR 水合 + 分析脚本

```go
var (
    reSSRHydration = regexp.MustCompile(
        `window\.__(?:NEXT_DATA__|NUXT__|INITIAL_STATE__|DATA__|RENDER_DATA__|PRELOADED_STATE__)\s*=\s*\{[\s\S]*?\}\s*;?`,
    )
    reAnalyticsScript = regexp.MustCompile(
        `(?:var\s+)?(?:_hmt|_gaq|gtag|fbq|_paq|wa_t)\s*[=\(][\s\S]*?</script>`,
    )
)

func normalizeInlineScript(content string) string {
    content = reSSRHydration.ReplaceAllString(content, "__SSR_HYDRATION__")
    content = reAnalyticsScript.ReplaceAllString(content, "__ANALYTICS__")
    return content
}

// computeScriptHash 中：
content = normalizeInlineScript(s.Text())
```

#### 0.3c `cleanHTML` — 仅维护现有职责

保留 `cleanHTML()` 当前功能，可持续微调，但不作为 Phase 0 主力。新增模式限于已验证有效的：

```
+ 时间戳: \d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})?
  → TIMESTAMP_REMOVED
+ UUID: [0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}
  → UUID_REMOVED
```

> 这些新增模式不改变 cleanHTML 的调用位置，只扩展其匹配能力。

### Step 0.4：httptest.Server 稳定复现测试

**不做**：把 baidu.com 作为必过单测（外部站点随时改版，测试脆弱且不可重现）。

**做**：用 `httptest.Server` 构造 4 类动态站点模板：

```go
// 测试 1: 时间戳波动 → 应 normal/normal_dynamic
func TestRelaxed_TimeBasedDynamicContent_NoFalsePositive(t *testing.T) {
    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintf(w, `<html><head></head><body>
            <main><p>当前时间: %d</p></main>
            <script>var _hmt = _hmt || []; _hmt.push(['_trackPageview', %d]);</script>
        </body></html>`, time.Now().Unix(), time.Now().UnixNano())
    }))
    defer ts.Close()
    // ... 基线 → 检测 → assert.False(t, result.Tampered)
}

// 测试 2: 版本化 JS 文件名 → 应 normal/normal_dynamic
func TestRelaxed_VersionedJSFiles_NoFalsePositive(t *testing.T) { /* ... */ }

// 测试 3: 新增/移除真实恶意脚本 → 应 tampered
func TestRelaxed_InjectedMaliciousScript_DetectsTamper(t *testing.T) { /* ... */ }

// 测试 4: 页面标题被替换 → 应 tampered
func TestRelaxed_TitleReplaced_DetectsTamper(t *testing.T) { /* ... */ }
```

baidu.com 真实站点验证保留为**手工测试**或 **nightly 集成测试**，不在单元测试中依赖。

### Step 0.5：ThresholdManager 移出 Phase 0

**不接入**。延后原因：

1. 阈值调整公式有 bug：`BaseThreshold * (1 - AdjustmentFactor)` 在 AdjustmentFactor 增加时**降低阈值**，误报率高时反而更容易告警
2. Detector 生命周期每次请求新建（`TamperAppService.newDetector()`），ThresholdManager 不持久
3. 无用户反馈回路（前端的"这不是篡改"按钮不存在），无法区分真/假阳性

延后至三个前置条件满足：
- 前端"标记误报"按钮 + API
- 阈值公式修正（`*(1+factor)` 或调整语义定义）
- ThresholdManager 挂在 TamperAppService 层跨请求共享

---

## 3. urlive.py 参考特性（后续 Phase）

| Phase | 内容 | 依赖 | 预估改动 |
|-------|------|------|---------|
| **1** | 指纹识别引擎：100+ 规则 YAML + Go 匹配引擎 + 集成到 PageHashResult | Phase 0 | ~400 行新增 |
| **2** | 规范化 HTTP 指纹：对 status_line + 排序头 + body 的组合做 MD5（**非** urlive.py 原始响应 MD5） | Phase 1（共享响应头收集） | ~80 行 |
| **3** | UA 池轮换 + SSL 跳过配置项 + 重定向记录 | Phase 0 | ~60 行 |
| **4** | 端口变更联动（长期项） | Phase 0-2 | 待评估 |

### Phase 1 要点：指纹识别引擎

- 新建 `internal/tamper/fingerprint/` 包
- `rules.yaml` 用 `//go:embed` 编译进二进制，100+ 规则从 urlive.py 移植
- 指纹运行结果附加到 `PageHashResult.Fingerprints`
- 指纹变化检测：新技术栈出现/消失 → 附加信息，不影响 `Tampered` 判定
- 高风险指纹变化（如 phpMyAdmin 新增、WAF 消失）→ 独立告警

### Phase 2 要点：规范化 HTTP 指纹

- **明确定义**：这是规范化 HTTP 指纹（`MD5(statusLine + sortedNormalizedHeaders + bodyHash)`），**不是** urlive.py 的原始响应 MD5
- 与 urlive.py 的指纹值不互通，但更适合对比变更场景（排除 Date/Age/Expires 等易变头）
- 字段名建议：`NormalizedHTTPFingerprint` 而非 `HTTPFingerprint`

### Phase 3 要点：UA 池等

- UA 在 `computeHashWithHTTP()` 中随机选择
- `InsecureSkipVerify` 作为 `DetectorConfig` 可选字段，默认 false
- `FinalURL` 记录重定向终点

---

## 4. 受影响的文件

### 修改

```
internal/tamper/detector_check.go     — SimpleMD5Hash 判定链路 + 状态语义修正
internal/tamper/detector_hash.go      — 分段 hasher 内部归一化 (js_files/scripts)
internal/service/tamper_app_service.go — summary 增加 normal_dynamic 统计
```

### 新增

```
internal/tamper/detector_relaxed_test.go — httptest.Server 4 类测试
```

### 不改

```
internal/tamper/detector_types.go     — 数据结构不变（Phase 0 不新增字段）
internal/tamper/detector.go           — Detector 结构体不变
internal/tamper/analyzer/*            — 不变
internal/tamper/threshold/*           — 不改，Phase 0 不接入
```

---

## 5. 验证清单

### 自动化测试

- [ ] `TestRelaxed_TimeBasedDynamicContent_NoFalsePositive` — 时间戳+分析脚本页面不误报
- [ ] `TestRelaxed_VersionedJSFiles_NoFalsePositive` — 版本化 JS 文件名不误报
- [ ] `TestRelaxed_InjectedMaliciousScript_DetectsTamper` — 注入恶意脚本应检出
- [ ] `TestStrict_SamePage_DetectsTamper` — strict 模式仍可检测时间戳变化
- [ ] 现有 `internal/tamper/` 下 22 个测试全部通过
- [ ] `go test -race ./internal/tamper/...` 通过

### 手工验证

- [ ] baidu.com Balanced + Relaxed 基线后立即检测 → `normal` 或 `normal_dynamic`，`Tampered=false`
- [ ] baidu.com 手工修改 static 页面内容 → `tampered`，`Tampered=true`

---

## 6. 回滚方案

每 Step 独立 commit，出问题 `git revert` 对应 commit：

```
Step 0.1 commit: fix(tamper): demote SimpleMD5Hash from veto to auxiliary signal in relaxed mode
Step 0.2 commit: fix(tamper): fix status semantics, normal_dynamic no longer sets Tampered=true
Step 0.3 commit: fix(tamper): normalize versioned assets and SSR hydration in segment hashers
Step 0.4 commit: test(tamper): add httptest-based false positive regression tests
```

---

> **v1.0 (原始版)**: 2026-06-25，初版计划  
> **v2.0 (修正版)**: 2026-06-25，7 问题核实 + 5 建议采纳后重写  
> **核心修正**: Phase 0 从"cleanHTML 增强 + forms token"重定向为"SimpleMD5Hash 链路 + 分段 hasher 内部归一化 + 状态语义修正"
