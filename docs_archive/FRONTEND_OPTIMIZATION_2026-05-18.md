# 前端统一优化记录

> 日期：2026-05-18 | 分支：`release/major-upgrade-vNEXT`
> 触发：`/frontend-design` — "优化当前项目的前端，注意前后端逻辑线条，统一样式，更加美化"

---

## 一、问题背景

优化前系统存在的 UI 问题：

1. **按钮风格不一致** — 大量 Emoji 图标（📄📋🔍🔄📷🗑️🏠⚙️📤📌📡💾🧹✅🗑️❌）与纯文本混用，视觉噪音大
2. **缺乏统一设计语言** — 按钮、卡片、表单、表格等组件无统一样式，各页面风格割裂
3. **Header 过于简陋** — 仅有文字链接，无品牌标识，导航不够清晰
4. **按钮类名混乱** — `btn`、`btn-sm`、`btn-info` 等类名混用，缺乏语义化分类
5. **Quota 页面结构 Bug** — `</nav></header>` 残留在模板中导致 HTML 结构断裂
6. **ICP 查询区域冗余** — 复选框 + 下拉框的双重控制模式（已在之前 session 改为 Tab）
7. **账号安全页单调** — 纯卡片堆叠，无视觉层次

---

## 二、设计方向

**"Industrial Security Console"** — 面向网络安全工具的工业控制台风格

| 维度 | 选择 |
|------|------|
| 主色调 | Teal（`#0d9488`）+ Sky Blue（`#0ea5e9`） |
| Header | 深色 Slate（`#0f172a`），sticky 定位 |
| Body 背景 | 浅灰 Slate（`#f1f5f9`） |
| 字体 | Inter（无衬线）+ JetBrains Mono（等宽） |
| 按钮体系 | Solid（主操作）+ Outline（次操作）双轨 |
| 卡片 | 白色底 + 圆角 + 悬停阴影 |
| 状态标识 | 语义化 Badge（成功绿 / 失败红 / 警告黄） |

---

## 三、变更清单

### 3.1 全局 CSS 重写（`web/static/css/style.css`）

| 模块 | 内容 |
|------|------|
| `:root` 设计令牌 | 颜色变量（brand/primary/success/danger/warning/info/neutral）、字体、间距、圆角、阴影、过渡 |
| Header 组件 | `.app-header`、`.header-inner`、`.header-brand`、`.header-logo`、`.header-tagline`、`.nav-bar` |
| 按钮体系 | `.btn`、`.btn-primary`、`.btn-secondary`、`.btn-success`、`.btn-danger`、`.btn-warning` + 对应的 `btn-outline-*` 变体 |
| 表单组件 | `.form-group`、`input`、`select`、`textarea`、`.code-editor`、`.editor-toolbar` |
| 引擎选择器 | `.engine-selector`、`.engine-option`、`.engine-status` |
| ICP Tab | `.icp-tabs`、`.icp-tab`、`.icp-tab.active` |
| 登录状态栏 | `.login-status-bar`、`.status-indicator` |
| Cookie 区域 | `.cookie-input-section`、`.cookie-input-content`、`.btn-toggle-cookie` |
| 模态框 | `.modal-overlay`、`.modal`、`.modal-content`、`.modal-header`、`.modal-footer` |
| 结果页 | `.results-table`、`.results-header`、`.payload-preview`、`.export-options`、`.filter-options` |
| 表格 | `.results-table`、hover 效果、状态码颜色 |
| 统计卡片 | `.stats-grid`、`.stat-item` |
| 进度条 | `.progress-bar`、`.progress-bar-fill` |
| 消息提示 | `.message`、`.alert`、`.toast` |
| 配额页 | `.quota-grid`、`.quota-item`、`.quota-bar`、`.quota-progress` |
| 定时任务页 | `.summary-cards`、`.tabs`、`.tab-content`、`.create-form` |
| 安全面板 | `.security-grid`、`.security-card`、`.status-badge` |
| 响应式 | `@media (max-width: 768px)`、`@media (max-width: 1024px)` |

**统计**：~1000 行（原 ~2291 行，精简但覆盖更广）

### 3.2 布局模板重构（`web/templates/layout.html`）

**Header 变化**：
- 添加 `.header-logo` 品牌标识（"U" 方块徽章）
- `.header-title-group` 含标题 + 副标题
- 导航简化为：查询 / 配额 / 截图 / 监控 / 任务 / 安全
- Sticky 定位 + 深色背景

**Footer 变化**：
- 居中布局 + 分隔符
- 品牌名 + 版权信息

### 3.3 模板文件 Emoji 清理

在以下 8 个模板文件中移除所有 Emoji 图标，替换为纯文本标签：

| 文件 | 替换项数 | 示例 |
|------|----------|------|
| `index.html` | 10 | `&#128269; 执行查询` → `执行查询` |
| `results.html` | 9 | `📄 导出CSV` → `导出CSV` |
| `quota.html` | 5 | `🔄 刷新配额` → `刷新配额` |
| `monitor.html` | 8 | `🔍 篡改检测` → `篡改检测` |
| `batch-screenshot.html` | 4 | `📷 开始批量截图` → `开始批量截图` |
| `scheduler.html` | 5 | `✎` → `编辑` |
| `error.html` | 1 | `🏠 返回首页` → `返回首页` |
| `account-security.html` | 已在上个 session 处理 | ✅ ❌ → 状态 Badge |

**注意**：`index.html` 使用 HTML 数字实体编码（如 `&#128269;`），其他文件使用 UTF-8 Emoji 字符。

### 3.4 按钮类名统一

| 场景 | 旧类名 | 新类名 |
|------|--------|--------|
| 主操作按钮 | `class="btn"` | `class="btn btn-outline-primary"` |
| 提交按钮 | `class="btn"` | `class="btn btn-primary"`（solid） |
| 表格内操作 | `class="btn btn-sm btn-info"` | `class="btn-sm btn-outline-primary"` |
| 成功操作 | `class="btn btn-outline-success"` | 保持不变 |
| 危险操作 | `class="btn btn-outline-danger"` | 保持不变 |
| Cookie 操作 | `class="btn btn-sm"` | `class="btn-sm btn-outline-secondary"` |
| 清空操作 | `class="btn btn-sm"` | `class="btn-sm btn-outline-danger"` |
| 导航链接 | `class="btn"` | `class="btn btn-outline-primary"` |

### 3.5 结构性修复

**Quota 页面 Bug**：
- 修复前：`</nav></header>` 残留在模板第 3-4 行，导致 HTML 结构断裂
- 修复后：仅保留 `{{template "header" ...}}` 和 `<main>`

### 3.6 JavaScript 更新（`web/static/js/main.js`）

- 重写 `initICPQuery()` 函数
- 替换复选框事件监听为 Tab 点击处理
- 添加 Tab 切换时的 placeholder 动态更新
- 移除下拉框同步逻辑

### 3.7 账号安全页重构（`web/templates/account-security.html`）

- `.security-grid` 网格布局（`grid-template-columns: repeat(auto-fit, minmax(350px, 1fr))`）
- 三个卡片区域：修改密码 / 认证状态 / Chrome 扩展桥接
- `.status-badge` 替代 ✅ ❌ Emoji
- 内联 `<style>` ~240 行（可后续提取至主 CSS）

---

## 四、技术细节

### 4.1 Emoji 替换方案

由于 UTF-8 多字节字符在不同工具中的兼容性差异，最终采用 Python 脚本处理：

```python
# HTML 实体编码（index.html）
content.replace('&#128269; 执行查询', '执行查询')

# UTF-8 Emoji（其他文件）
content.replace('🔄 刷新配额', '刷新配额')

# 正则替换按钮类名（避免误替换 btn-primary 等）
re.sub(r'class="btn"(?![a-z-])', 'class="btn btn-outline-primary"', content)
```

### 4.2 按钮类名正则

```regex
class="btn"(?![a-z-])
```
确保只匹配独立的 `btn`，不会匹配 `btn-primary`、`btn-sm` 等变体。

### 4.3 设计令牌（CSS Variables）

```css
:root {
  /* 品牌色 */
  --color-brand: #0d9488;        /* Teal */
  --color-brand-light: #14b8a6;
  --color-brand-dark: #0f766e;

  /* 功能色 */
  --color-primary: #0ea5e9;      /* Sky Blue */
  --color-success: #10b981;      /* Emerald */
  --color-danger: #ef4444;       /* Red */
  --color-warning: #f59e0b;      /* Amber */

  /* 背景 */
  --color-header-bg: #0f172a;    /* Dark Slate */
  --color-body-bg: #f1f5f9;      /* Light Slate */
  --color-surface: #ffffff;

  /* 字体 */
  --font-sans: 'Inter', -apple-system, ...;
  --font-mono: 'JetBrains Mono', 'Fira Code', ...;

  /* 间距 */
  --space-xs: 4px;
  --space-sm: 8px;
  --space-md: 16px;
  --space-lg: 24px;
  --space-xl: 32px;

  /* 圆角 */
  --radius-sm: 4px;
  --radius-md: 8px;
  --radius-lg: 12px;

  /* 阴影 */
  --shadow-sm: 0 1px 2px rgba(0,0,0,0.05);
  --shadow-md: 0 4px 6px rgba(0,0,0,0.1);
  --shadow-lg: 0 10px 15px rgba(0,0,0,0.1);
}
```

---

## 五、影响范围

| 页面 | 受影响区域 | 变化类型 |
|------|------------|----------|
| `/`（首页） | Header、查询表单、按钮、ICP 区域 | 样式 + 结构 + 按钮类名 |
| `/query`（结果页） | 导出按钮、筛选按钮、表格操作 | 按钮类名 + Emoji 清理 |
| `/quota`（配额页） | Header 修复、操作按钮、进度条 | 结构修复 + 样式 |
| `/monitor`（监控页） | 操作按钮、Tab 切换 | 按钮类名 + Emoji 清理 |
| `/batch-screenshot`（批量截图） | 操作按钮 | 按钮类名 + Emoji 清理 |
| `/scheduler`（定时任务） | 表格操作按钮 | 按钮类名 + Emoji 清理 |
| `/account-security`（账号安全） | 整个页面重构 | 全新布局 |
| `/login`（登录页） | 无变化 | 仅继承全局 CSS |
| `/error`（错误页） | 返回首页按钮 | Emoji 清理 |

---

## 六、验证结果

- **构建状态**：`go build ./...` 通过
- **模板完整性**：全部 10 个模板文件行数正常
- **Emoji 清理**：全部 8 个文件已无 Emoji 字符
- **CSS 加载**：新样式已正确服务（HTTP 200）
- **服务运行**：端口 8448 正常响应

---

## 七、后续建议

1. **提取内联样式** — `account-security.html` 的 ~240 行 `<style>` 可提取至主 CSS 文件
2. **响应式测试** — 在 320px/768px/1024px/1440px 四个断点验证布局
3. **暗色模式** — 当前为亮色主题，可考虑添加暗色模式支持
4. **可访问性** — 检查对比度（WCAG AA 标准）、键盘导航、ARIA 标签
5. **性能优化** — CSS 文件可进一步压缩，考虑按需加载非关键样式
6. **图标系统** — 如果需要图标，建议使用 SVG sprite 或 icon font，而非 Emoji
