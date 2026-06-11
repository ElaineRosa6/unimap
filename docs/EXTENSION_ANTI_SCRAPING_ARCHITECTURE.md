# Extension 反爬虫架构分析与实施方案

> 日期：2026-06-05 | 更新：2026-06-10 | 分支：release/major-upgrade-vNEXT

## 一、背景

UniMap 需要从五大安全搜索引擎（FOFA、Hunter、ZoomEye、Shodan、Quake）提取搜索结果数据。
核心问题：这些引擎有不同程度的反爬虫机制，如何稳定地提取数据？

## 二、当前架构概述

### 数据流

```
后端 Service → ScreenshotRouter → ExtensionProvider → BridgeService(队列+重试)
    → HTTP 轮询 (1s) → Chrome Extension handleTask()
    → DOM 提取 (ENGINE_SELECTORS + fallback 链)
    → HMAC-SHA256 签名回传 → 后端归并去重
```

### 关键组件

| 组件 | 文件 | 职责 |
|------|------|------|
| ScreenshotRouter | `internal/screenshot/router.go` | 双模式健康检查 + 自动降级 |
| BridgeService | `internal/screenshot/bridge_service.go` | 队列 + worker 池 + 重试 |
| BridgeTask/BridgeResult | `internal/screenshot/bridge_types.go` | 共享契约类型 |
| background.js | `tools/extension-screenshot/src/background.js` | 轮询循环 + 任务分发 + Token 管理 |
| capture.js | `tools/extension-screenshot/src/capture.js` | 标签页管理 + 页面等待 + DOM 提取 |
| api.js | `tools/extension-screenshot/src/api.js` | HMAC-SHA256 签名 HTTP 客户端 |

### 两种模式对比

| 维度 | CDP 模式 | Extension 模式 |
|------|----------|----------------|
| `navigator.webdriver` | `true` ❌ | `false` ✅ |
| `chrome.runtime` | 不存在 ❌ | 正常存在 ✅ |
| TLS 指纹 | 非标准 ❌ | 真实 Chrome ✅ |
| Canvas 指纹 | headless 差异 ❌ | 真实渲染 ✅ |
| Cookie/Session | 需手动注入 ❌ | 用户真实会话 ✅ |
| 行为模式 | 程序化操作 ❌ | 可模拟人类 ✅ |
| 扩展检测 | 无扩展 ❌ | 可安装正常扩展 ✅ |

**结论：Extension 模式是正确方向，应作为主力模式。**

## 三、各引擎反爬难度分析

| 引擎 | 反爬强度 | 主要防线 | Extension 可行性 |
|------|----------|----------|-----------------|
| **FOFA** | 🟡 中等 | 登录墙 + 频率限制 + 前端混淆 class | ✅ 可行，需登录态 |
| **Hunter** | 🟡 中等 | 登录墙 + 每日配额 + 动态 class | ✅ 可行，需登录态 |
| **ZoomEye** | 🟡 中等 | 登录墙 + Cloudflare JS Challenge | ⚠️ 需处理 CF challenge |
| **Shodan** | 🔴 较高 | Cloudflare WAF + TLS 指纹 + 行为分析 | ⚠️ 需要 stealth |
| **Quake** | 🔴 较高 | 360 安全体系 + 验证码 + 行为分析 | ⚠️ 需要 stealth + 验证码处理 |

## 四、当前代码中的反爬缺陷

### CDP 模式（高度暴露）

```go
// internal/screenshot/manager.go:473-521
opts := append(chromedp.DefaultExecAllocatorOptions[:],
    chromedp.Flag("headless", m.headless),       // ← headless 指纹
    chromedp.Flag("disable-gpu", true),           // ← 自动化标志
    chromedp.Flag("no-sandbox", true),
    // 缺少: --disable-blink-features=AutomationControlled
    // 缺少: excludeSwitches: ["enable-automation"]
    // navigator.webdriver = true (默认)
)
```

### Extension 模式（无显式 stealth）

当前 Extension 代码无任何反检测机制：
- 无 User-Agent 旋转
- 无视口随机化
- 无请求时间抖动
- 无 WebDriver 标志隐藏
- 无指纹随机化

唯一的"隐性"优势：使用用户真实浏览器会话。

## 五、实施方案

### 阶段 1：Stealth 脚本注入

在页面加载前注入反检测脚本：

```javascript
// stealth.js — 通过 chrome.scripting.executeScript 在页面加载前注入
const STEALTH_SCRIPTS = [
  // 隐藏 webdriver 标志
  `Object.defineProperty(navigator, 'webdriver', {get: () => false})`,

  // 伪造 plugins（headless Chrome 默认无插件）
  `Object.defineProperty(navigator, 'plugins', {
    get: () => [1, 2, 3, 4, 5].map(() => ({ length: 1 }))
  })`,

  // 伪造 languages
  `Object.defineProperty(navigator, 'languages', {
    get: () => ['zh-CN', 'zh', 'en-US', 'en']
  })`,

  // 修改 permissions API 行为
  `const originalQuery = window.Permissions.prototype.query;
   window.Permissions.prototype.query = (parameters) =>
     parameters.name === 'notifications'
       ? Promise.resolve({ state: Notification.permission })
       : originalQuery(parameters)`,

  // 隐藏自动化 iframe
  `Object.defineProperty(HTMLIFrameElement.prototype, 'contentWindow', {
    get: function() {
      if (this.src && this.src.includes('utomation')) return undefined;
      return Object.getOwnPropertyDescriptor(
        HTMLIFrameElement.prototype, 'contentWindow'
      ).get.call(this);
    }
  })`
];
```

### 阶段 2：各引擎选择器维护

当前 `capture.js` 中的 `ENGINE_SELECTORS` 已有框架，需为每个引擎维护完整选择器：

```javascript
// 示例结构（需根据实际 DOM 调整）
shodan: {
  row: [
    'table.table > tbody > tr',      // 经典表格
    '.search-result',                 // 卡片布局
    '[class*="result"] tr',          // 模糊匹配
  ],
  cells: {
    ip:   { selector: 'td:first-child a', extract: 'text' },
    port: { selector: 'td:nth-child(2)',  extract: 'text' },
    host: { selector: 'td:nth-child(3)',  extract: 'text' },
  },
  total: ['.results-header', '[class*="total"]'],
  nextPage: ['a[rel="next"]', '.pagination .next'],
  blocked: ['.cf-browser-verification', '#challenge-form', '.captcha'],
}
```

选择器应遵循：
- 每个引擎至少 3 个 row selector fallback
- 使用属性选择器 `[class*="xxx"]` 应对动态 class
- 添加 `blocked` 字段检测拦截页面
- 定期巡检更新（引擎前端改版频率高）

### 阶段 3：智能等待策略

替代当前的固定 4000ms 延时：

```javascript
async function waitForResults(tabId, engine, timeout = 15000) {
  const start = Date.now();
  const selectors = ENGINE_SELECTORS[engine];

  while (Date.now() - start < timeout) {
    const [{ result }] = await chrome.scripting.executeScript({
      target: { tabId },
      func: (sel) => {
        // 检测结果是否已渲染
        for (const rowSel of sel.row) {
          const rows = document.querySelectorAll(rowSel);
          if (rows.length > 0) return { ready: true, count: rows.length };
        }
        // 检测是否被拦截
        for (const blockSel of (sel.blocked || [])) {
          if (document.querySelector(blockSel)) return { blocked: true };
        }
        return { ready: false };
      },
      args: [selectors]
    });

    if (result.blocked) throw new Error('BLOCKED_BY_ANTI_BOT');
    if (result.ready) return result.count;

    await new Promise(r => setTimeout(r, 500));
  }
  throw new Error('WAIT_TIMEOUT');
}
```

### 阶段 4：速率控制

```javascript
const RATE_LIMITS = {
  fofa:    { qps: 2,   burst: 5,  cooldown: 30000  },
  hunter:  { qps: 1,   burst: 3,  cooldown: 60000  },
  zoomeye: { qps: 1,   burst: 3,  cooldown: 60000  },
  shodan:  { qps: 0.5, burst: 2,  cooldown: 120000 },
  quake:   { qps: 0.5, burst: 2,  cooldown: 120000 },
};

class TokenBucket {
  constructor(qps, burst) {
    this.tokens = burst;
    this.maxTokens = burst;
    this.refillRate = qps;
    this.lastRefill = Date.now();
  }
  tryConsume() {
    this.refill();
    if (this.tokens >= 1) { this.tokens--; return true; }
    return false;
  }
  refill() {
    const now = Date.now();
    this.tokens = Math.min(this.maxTokens,
      this.tokens + (now - this.lastRefill) / 1000 * this.refillRate);
    this.lastRefill = now;
  }
}
```

### 阶段 5：验证码处理

```javascript
async function detectCaptcha(tabId) {
  const [{ result }] = await chrome.scripting.executeScript({
    target: { tabId },
    func: () => {
      const body = document.body.innerText;
      const indicators = [
        /验证码/i, /captcha/i, /recaptcha/i,
        /hcaptcha/i, /turnstile/i,
        /verify.*human/i, /人机验证/i,
        /安全验证/i, /cf-challenge/i,
      ];
      return indicators.some(re => re.test(body));
    }
  });
  return result;
}

// 检测到验证码时：通知后端 + 等待用户手动解决
if (await detectCaptcha(tabId)) {
  await reportTaskResult({
    status: 'captcha_required',
    message: '检测到验证码，需要手动解决',
    screenshot: await captureVisibleTab(),
  });
  await waitForCaptchaResolution(tabId, 60000);
}
```

## 六、CDP 能完全避免检测吗？

**不能。** 即使加了所有 stealth 手段：

| 检测手段 | 绕过难度 | 说明 |
|----------|----------|------|
| `navigator.webdriver` | 🟡 可绕过 | 但 `Object.getOwnPropertyDescriptor` 可检测原型链篡改 |
| CDP Runtime.enable | 🔴 难绕过 | 会注入 `__cdp_evaluation_id__` 等痕迹 |
| TLS 指纹 (JA3/JA4) | 🔴 难绕过 | Go 的 TLS 栈与 Chrome 有差异 |
| Canvas/WebGL 指纹 | 🔴 难绕过 | headless 渲染与真实 Chrome 有像素级差异 |
| 行为分析 | 🟡 可模拟 | 需要随机延时、鼠标轨迹、滚动模拟 |
| Cloudflare Bot Management | 🔴 难绕过 | 200+ 种自动化特征综合检测 |

**建议：CDP 仅作降级备选，不投入大量 stealth 开发。**

## 七、最佳实践

### 应该做的

1. **Extension 模式作为主力**，CDP 仅作降级备选
2. **添加 stealth 注入**，至少覆盖 webdriver、plugins、languages
3. **实现智能等待**替代固定延时
4. **添加速率控制**，每个引擎独立限流
5. **验证码检测 + 通知**，允许用户手动介入
6. **选择器 fallback 链**，应对前端改版
7. **定期巡检**各引擎选择器有效性

### 不应该做的

1. ❌ 不要在 CDP 上投入大量 stealth 工作 — 收益递减
2. ❌ 不要尝试自动化绕过验证码 — 法律风险高，成功率低
3. ❌ 不要高频轮询同一引擎 — 触发封禁比获取数据更快
4. ❌ 不要硬编码选择器 — 使用 fallback 链 + 模糊匹配

## 八、完整数据流（改进后）

```
用户查询请求
    │
    ▼
后端 Service 层
    │
    ▼
ScreenshotRouter.resolveProvider()
    ├── Extension 健康？ → ExtensionProvider
    │       │
    │       ▼
    │   BridgeService.Submit(task{action:"collect", url, engine})
    │       │
    │       ▼
    │   HTTP 队列 (pending map)
    │       │  ← Extension bridgeLoop() 每 1s 轮询
    │       ▼
    │   Extension handleTask()
    │       │
    │       ├─ 1. 注入 stealth 脚本
    │       ├─ 2. ensureTab() → 打开/复用标签页
    │       ├─ 3. 导航到 URL
    │       ├─ 4. waitForResults() 智能等待
    │       │      ├─ 成功 → 提取数据
    │       │      ├─ 验证码 → 通知后端 + 等待人工
    │       │      └─ 被拦截 → 报告 blocked 状态
    │       ├─ 5. extractEngineAssets() DOM 提取
    │       └─ 6. reportTaskResult() HMAC 签名回传
    │
    ▼
后端收到 UnifiedAsset[] → 归并去重 → 返回用户
```

## 九、涉及的关键文件

| 文件 | 改动类型 | 说明 |
|------|----------|------|
| `tools/extension-screenshot/src/capture.js` | 修改 | 添加 stealth 注入 + 智能等待 + blocked 检测 |
| `tools/extension-screenshot/src/background.js` | 修改 | 添加速率控制 + 验证码处理逻辑 |
| `internal/screenshot/manager.go` | 修改 | CDP 模式添加 stealth flags（低优先级） |
| `internal/screenshot/dom_selectors.go` | 修改 | Go 侧选择器与 Extension 侧同步 |
| `configs/config.yaml.example` | 修改 | 添加速率限制配置项 |

## 十、实施排期（含 Quake 验证）

> 制定日期：2026-06-07。**前置认知更正**：Quake 采集失败是 360 反爬检测拦截（用户手动查询正常，账号有搜索权限），**不是账号无权限**。因此 Quake 属于 stealth 可改善范畴，验证应排在 stealth 强化之后，而非搁置等账号升级。详见 `docs/E2E_COLLECTION_VERIFICATION_2026-06-04.md` §4.2。

### 阶段顺序与 Quake 验收点

| 阶段 | 内容 | Quake 相关验收 | 状态 |
|------|------|---------------|------|
| 阶段 1 | Stealth 脚本注入（webdriver/plugins/languages/permissions） | Extension 打开 Quake 搜索页，`navigator.webdriver===false`，页面不再返回"用户缺少必要权限"拦截页 | ⏸️ 待实施 |
| 阶段 2 | 各引擎选择器维护（含 `blocked` 检测） | Quake 加入 `ENGINE_SELECTORS`：row + cell 选择器 + `blocked` 拦截页检测（"安全验证"/"缺少必要权限"） | ⏸️ 待实施 |
| 阶段 3 | 智能等待 `waitForResults` | Quake SPA（Element UI）结果渲染后再提取，替代固定延时；命中 `blocked` 抛 `BLOCKED_BY_ANTI_BOT` | ⏸️ 待实施 |
| 阶段 4 | 速率控制（令牌桶，`quake: qps 0.5 / cooldown 120s`） | Quake 限流配置生效，避免高频触发 360 封禁 | ⏸️ 待实施 |
| 阶段 5 | 验证码检测 + 通知人工介入 | Quake 触发"安全验证/人机验证"时 `detectCaptcha` 命中 → 上报 `captcha_required` + 截图 → 等待人工 | ⏸️ 待实施 |

### Quake 端到端验收（阶段 1-3 完成后执行）

```bash
# 前置：Chrome 已登录 Quake（360U3166720809）+ Extension 已加载已配对 + 服务器运行
# 通过 Web UI 或 API 触发 Quake 采集
ADMIN="<admin_token>"
curl -s -X POST "http://127.0.0.1:8448/api/v1/query" \
  -H "Authorization: Bearer $ADMIN" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -H "Origin: http://localhost:8448" \
  --data-urlencode 'query=ip="X.X.X.X"' \
  --data-urlencode 'engines=quake' \
  --data-urlencode 'browser_query=true' \
  --data-urlencode 'browser_action=collect'
```

**通过判据**：
1. 返回页面标题正确（"360网络空间测绘"）且**非拦截页**（无"用户缺少必要权限"/"暂无数据"）
2. `structured_collected_data` 提取出 ≥1 条资产（IP/Port/字段）
3. 与用户手动查询结果一致

**若仍被拦截**：记录命中的反爬特征，回到阶段 1 补强 stealth；360 体系较强，可能需要行为模拟（随机延时/滚动）作为阶段 1 的扩展项。

### 与三层采集架构的关系

Quake 在三层架构 H-3「强反爬引擎 L1/L2 归零」中属于 **stealth 可改善**范畴，不是无解项。本 stealth 方案是 Quake（及 Shodan）能否走 L1 Network / L2 Hook 层的前提——stealth 不到位时，三层全部退回 DOM 也无济于事。建议 **stealth 阶段 1-3 先行，再评估 Quake/Shodan 的三层采集可行性**。相关：`docs/THREE_LAYER_COLLECTION_ARCHITECTURE.md` 第 7、9 节。

## 十一、后续实施评估与执行方案

> 更新日期：2026-06-10。该项剩余工作依赖真实浏览器、真实登录态和目标站当前反爬策略，不能只靠单元测试闭环。实施时按阶段开关推进，每阶段都必须能回退到现有 Extension DOM 提取或 CDP 降级路径。

### 11.1 工作量与风险评估

| 阶段 | 预估工作量 | 主要风险 | 风险等级 | 是否依赖真机 |
|------|------------|----------|----------|--------------|
| 阶段 1 Stealth 注入 | 1-2 天 | 注入时机晚于站点检测；脚本特征本身被识别 | 高 | 是 |
| 阶段 2 blocked/login/captcha 检测 | 1-2 天 | 不同站点提示文案和页面结构变化快 | 中 | 是 |
| 阶段 3 智能等待 | 1-2 天 | SPA 空状态、慢响应、虚拟列表难区分 | 中 | 是 |
| 阶段 4 速率控制 | 1 天 | 过严影响吞吐，过松触发封禁 | 中 | 否，最终需真机确认 |
| 阶段 5 验证码人工介入 | 2-3 天 | 用户体验与任务超时状态复杂 | 高 | 是 |

建议先完成阶段 1-3 并做 Quake smoke test，再决定是否继续阶段 4-5。阶段 4 配置可以先落地默认值，但真实阈值需要运行数据校准。

### 11.2 实施边界

- **主路径只做 Extension 真实浏览器**：优先修改 `tools/extension-screenshot/src/capture.js` 和 `background.js`，不把主要精力投入 CDP stealth。
- **注入范围必须收窄**：`chrome.scripting.executeScript` 只对已配置引擎域名执行，保持当前域名白名单策略，不回退到 `<all_urls>`。
- **注入时机尽量提前**：stealth 脚本在 `document_start` 或导航后第一时间执行；若 MV3 生命周期限制导致时机不稳定，需要记录实际触发顺序。
- **不自动破解验证码**：检测到验证码只上报 `CAPTCHA_REQUIRED`、截图、等待人工处理或超时。
- **不高频重试**：blocked/captcha/login 状态不进入普通重试循环，必须进入冷却或等待用户动作。

### 11.3 关键实现点

1. **Stealth 脚本**

   建议新增独立 `stealth.js` 或在 `capture.js` 中拆出 `injectStealth(tabId, engine)`。最小覆盖：

   - `navigator.webdriver` 返回 `false` 或 `undefined`
   - `navigator.plugins`/`navigator.languages` 与真实 Chrome 接近
   - `navigator.permissions.query` 对 notifications 等常见检测保持合理行为
   - 必要时补 `chrome.runtime`、WebGL vendor/renderer 的轻量兼容，不做复杂指纹伪造

2. **统一状态码**

   Extension 返回结果应区分以下状态，后端不要把它们折叠成普通失败：

   | 状态 | 含义 | 后续动作 |
   |------|------|----------|
   | `BLOCKED_BY_ANTI_BOT` | 命中反爬或风控拦截 | 记录特征，进入冷却，不自动重试 |
   | `CAPTCHA_REQUIRED` | 需要人机验证 | 截图并提示人工处理 |
   | `LOGIN_REQUIRED` | 登录态失效或账号未授权 | 提示用户重新登录 |
   | `NO_RESULTS` | 查询成功但无结果 | 作为正常空结果 |
   | `COLLECTED` | 已采集到结构化资产 | 进入归并去重 |

3. **`waitForResults` 语义**

   等待条件应是“任一终态出现”，而不是固定 sleep：

   - 结果行出现：提取并返回 `COLLECTED`
   - 明确空状态出现：返回 `NO_RESULTS`
   - 登录页/登录弹窗出现：返回 `LOGIN_REQUIRED`
   - 验证码或人机验证出现：返回 `CAPTCHA_REQUIRED`
   - 反爬拦截文案出现：返回 `BLOCKED_BY_ANTI_BOT`
   - 超时：返回带页面摘要和截图的 timeout error

4. **限流配置**

   初始建议：

   | 引擎 | qps | burst | cooldown |
   |------|-----|-------|----------|
   | Quake | 0.5 | 1-2 | 120s |
   | Shodan | 0.5 | 1-2 | 120s |
   | Hunter | 1 | 2-3 | 60s |
   | ZoomEye | 1 | 2-3 | 60s |
   | FOFA | 1-2 | 3-5 | 30-60s |

   配置落点建议在 `configs/config.yaml.example` 增加示例，并由 `background.js` 使用后端下发或本地默认值。连续 blocked 后应指数退避，但设置最大冷却，避免任务永久挂起。

### 11.4 测试计划

| 类型 | 内容 | 通过标准 |
|------|------|----------|
| 本地 mock 页面 | 构造结果页、空页、登录页、验证码页、blocked 页 | `waitForResults` 能返回正确终态 |
| Extension E2E | 已配对 Extension 拉取后端任务并回传状态 | HMAC 回传成功，任务状态不丢失 |
| Quake smoke test | 已登录真实 Chrome 执行 1 条低频查询 | 非拦截页且提取到结构化资产，或明确返回 blocked/captcha/login |
| Shodan smoke test | 已登录真实 Chrome 执行 1 条低频查询 | 不把 OR 降级等语法限制误判为采集失败 |
| 回归测试 | 普通截图、批量截图、已有 DOM 提取路径 | 未启用 stealth 时现有路径保持可用 |

真机测试必须使用低频、小批量、可审计的查询。测试记录应包含时间、引擎、查询语句、页面状态、是否触发 blocked/captcha、截图路径或任务 id。

### 11.5 回滚与观测

- 增加 feature flag，例如 `extension.stealth.enabled`、`extension.rate_limit.enabled`、`extension.captcha_wait.enabled`。
- 阶段 1-3 任一项导致正常引擎采集下降时，按引擎关闭，不全局回滚 Extension。
- 后端记录 `engine/status/reason/task_id/duration/cooldown`，便于区分“无结果”和“被拦截”。
- 前端或 API 结果中暴露可读状态，避免用户把 `CAPTCHA_REQUIRED`、`LOGIN_REQUIRED` 误解为系统错误。
