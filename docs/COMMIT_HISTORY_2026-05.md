# UniMap 提交记录文档（2026年5月）

> 统计周期：2026-05-06 ~ 2026-06-01
> 分支：`release/major-upgrade-vNEXT` → `develop`
> 作者：ElaineRosa6（部分提交 Co-authored-by: monkeycode-ai、Claude Opus 4.7）
> 总提交数：**77** 个（不含合并提交）

---

## 整体统计

| 类型 | 数量 | 占比 |
|------|------|------|
| feat（功能） | 19 | 24.7% |
| fix（修复） | 35 | 45.5% |
| docs（文档） | 17 | 22.1% |
| chore（杂项） | 3 | 3.9% |
| style（风格） | 3 | 3.9% |
| test（测试） | 1 | 1.3% |
| **合计** | **77** | **100%** |

### 按日分布

| 日期 | 提交数 |
|------|--------|
| 05-06 | 3 |
| 05-07 | 12 |
| 05-08 | 3 |
| 05-09 | 8 |
| 05-20 | 5 |
| 05-21 | 10 |
| 05-22 | 3 |
| 05-23 | 10 |
| 05-24 | 4 |
| 05-25 | 3 |
| 05-26 | 7 |
| 05-27 | 3 |
| 05-28 | 1 |
| 05-29 | 1 |
| 05-30 | 1 |
| 05-31 | 2 |
| 06-01 | 2 |

---

## 第一周（5月6日 — 5月9日）：安全加固 + 浏览器双模查询

**主题**：完成 4月29日深度代码审查的全部修复，上线登录认证系统、浏览器双模查询引擎（CDP/Extension）、前端统一化重构。

### 5月6日（3 提交）

| 提交 | 类型 | 说明 |
|------|------|------|
| `8446404` | fix | **修复 4月29日深度审查的剩余 11 项发现**<br>• AUTH: APIKeyManager 使用 SHA-256 哈希校验 + ID 索引<br>• SSRF: Webhook DNS 解析校验 + safeWebhookClient 拒绝重定向<br>• DIST: TaskQueue JSON 快照持久化<br>• BACKUP: 源/tar 错误累积收集不再静默吞掉<br>• RATELIMIT: X-Real-IP 代理信任检查<br>• CONFIG: 非本地绑定打印 admin_token、CORS 默认包含 X-Admin-Token<br>• CLI: writeJSONFile 使用 O_EXCL 防止覆盖<br>• 30 个测试包 -race 全绿 |
| `36011d3` | feat | **登录页面 + Session Cookie 认证 + WebSocket/状态修复**<br>• 用户名密码登录页面，AES-GCM Session Cookie<br>• WebSocket 认证从 X-Admin-Token 迁移到 Session Cookie（浏览器 WS API 无法设置自定义头）<br>• CDP/Extension 状态一致性：状态栏改用真实 HTTP 探测<br>• 导航栏 + 退出按钮、CSRF double-submit、bcrypt 密码哈希、登录速率限制<br>• 保留 X-Admin-Token 兼容 CLI/API<br>新增文件: `web/session.go`, `web/login_handlers.go`, `web/templates/login.html` |
| `8271a6f` | fix | **Bridge 认证冲突、browser_query 门控、双模截图**<br>• `/api/screenshot/bridge/` 加入公开路径（Bridge 自有认证）<br>• 前端 browser_query 同时检查 CDP/Extension 在线状态<br>• ScreenshotRouter 移除硬编码 ModeCDP，新增 SetMode()/CurrentMode()<br>• Provider 接口新增 OpenSearchEngineResult 方法 |

### 5月7日（12 提交）

| 提交 | 类型 | 说明 |
|------|------|------|
| `8be2de0` | feat | **Phase 2 双模浏览器执行抽象 + 模式选择器 UI + Collect 能力**<br>• Provider 接口扩展 CollectResult/CollectSearchEngineResult<br>• ModeAuto 常量 + SetMode 接受 cdp/extension/auto<br>• BridgeTask 增加 Action 字段（screenshot/open/collect）<br>• POST /api/screenshot/set-mode 运行时切换端点<br>• 前端查询表单增加 CDP/扩展/自动 单选按钮<br>• 模式选择 localStorage 持久化，自动同步到服务端 |
| `25e59b5` | feat | **Phase 3 浏览器映射主管道 — Web-Only 引擎 DOM 提取和统一结果归并**<br>• 创建 `dom_selectors.go`（FOFA/Hunter/ZoomEye/Quake 引擎 CSS 选择器）<br>• Manager.CollectSearchEngineResult 实现真实 DOM 提取<br>• WebOnlyAdapterBase 委托底层适配器翻译 UQL<br>• BrowserQueryBackend 接口 + orchestrator 注入<br>• Shodan 支持 |
| `f3e9c4c` | feat | **Phase 4 前端统一 — 共享布局模板、CSS 修复、导航一致性**<br>• 创建 `layout.html` 共享头部/导航/页脚<br>• 迁移 index/scheduler/monitor/batch-screenshot/results/quota 到布局模板<br>• 补全缺失 CSS 变量（--accent, --surface-elevated 等）<br>• 新增 dict 模板函数 |
| `be56674` | fix | 修复 layout head 模板缺失 .Title 的 fallback 默认值 |
| `f2dbac8` | fix | **允许 chrome-extension:// 源通过 CORS**（Extension Bridge 配对需要） |
| `c6fb942` | fix | **停止 Extension 登录状态轮询时打开引擎页面**（改为 browser_session 状态报告） |
| `281da22` | style | 改进 Cookie 输入区域视觉层次（折叠头、卡片容器、网格布局） |
| `37c2a6f` | style | 细化 Cookie 输入区域视觉（虚线边框、渐变背景、绿/灰状态点） |
| `137a027` | feat | **浏览器 Collect 动作 + 结构化资产提取 + 前端视觉收敛**<br>• browser_action 参数（open/capture/collect）含防崩溃降级<br>• 扩展 capture.js 引擎选择器重构<br>• batch-screenshot.html 孤儿标签修复 |
| `4cf876a` | docs | 更新整改计划状态 — 标记步骤 1-5 全部完成 |
| `992ed71` | fix | **修复第3轮代码审查的 13 项安全/质量问题**<br>• C-01: 空 admin_token 自动生成随机值<br>• C-03: RoundRobinScheduler 原子操作修复<br>• C-04: rateLimitEnabled 竞态修复<br>• H-01: CSP 移除 unsafe-eval<br>• M-02/M-03: 上传文件 MIME 校验 + 文件名净化<br>• 多处原子/并发/超时/日志修复 |
| `17ebf44` | fix | **Bridge 路由绕过 CORS 检查**（Bridge 有独立认证，不需要 CORS 限制） |

### 5月8日（3 提交）

| 提交 | 类型 | 说明 |
|------|------|------|
| `19a38b8` | chore | **格式对齐 + 默认启用全部引擎 + Bridge CORS 头**<br>• config.go 结构体字段对齐<br>• Quake/ZoomEye/Hunter/Fofa/Shodan 默认全部启用<br>• Fofa.UseWebAPI 默认启用<br>• CORS allowed headers 增加 Bridge 签名头 |
| `53a9b8d` | feat | **返回浏览器采集的查询数据**（与 monkeycode-ai 协作） |
| `3d13d41` | fix | **稳定浏览器扩展查询状态**（与 monkeycode-ai 协作） |

### 5月9日（8 提交）

| 提交 | 类型 | 说明 |
|------|------|------|
| `82c4100` | docs | **Extension 模式问题报告**（UQL 翻译、进度、采集、登录状态 4 个问题及根因分析） |
| `2044053` | feat | **capture.js v0.2.0 — 登录墙检测、SPA 支持、多选择器回退**<br>• ENGINE_SELECTORS 多选择器变体配置<br>• isLoginWall() 检测（FOFA/Hunter/ZoomEye/Quake）<br>• waitForPageReady SPA/networkidle 策略<br>• manifest.json → v0.2.0 |
| `4584e9d` | feat | **capture.js 升级 — 卡片式选择器 + 提取回退**<br>• FOFA 从表格选择器升级到 12 种选择器（卡片+混合+表格回退）<br>• href 模式匹配（IP/端口/协议）<br>• 新增测试脚本和 DOM 分析工具 |
| `738d5e1` | fix | **浏览器查询修复计划全部 6 项闭环**<br>• P0: 浏览器模式 UQL 翻译<br>• P0: 结构化采集资产合并<br>• P1: WebSocket 进度回调<br>• P1: Bridge 打开失败检测<br>• P1: 登录状态误判修复<br>• P2: Extension 字段补全<br>新增 5 个测试，-race 全绿 |
| `ed1c142` | docs | 提交 `738d5e1` 的代码审查验证报告 |
| `431cbac` | docs | **更新 CLAUDE.md 已知问题列表**（标记已修复、新增剩余项目） |
| `897173d` | fix | **默认启用 Bridge 签名 + Scheduler CSP nonce**（与 monkeycode-ai 协作） |
| `59f3a23` | fix | **Goroutine 泄漏保护 + CSRF 写端点覆盖**<br>• Pool.Stop() 增加 30s 超时<br>• ~20 个 POST/PUT/DELETE handler 增加 requireTrustedRequest<br>• 全部受影响测试增加 Origin 头 |

---

## 第二周（5月20日 — 5月26日）：ICP备案 + 通知系统 + 前端重设计 + v1.0.0

**主题**：10天空白期后（远程 rebase），ICP备案查询全链路、推送通知系统、前端工业极简重设计、v1.0.0 发布。

### 5月20日（5 提交）

| 提交 | 类型 | 说明 |
|------|------|------|
| `a4c8641` | docs | **综合代码审查报告（2026-05-19）**（与 monkeycode-ai 协作） |
| `6b5b23f` | feat | **远程 rebase 前本地变更同步** — 文档归档、模板更新、ICP 适配器变更 |
| `0406532` | fix | **验证修复 5 个代码审查问题**（P1-2, P1-4, P2-1, P2-7, P2-9, P2-10） |
| `c315ebf` | docs | **修复验证审计报告（2026-05-20）**<br>• 验证 FIX_REPORT 中 5 个 DONE 项实际未提交<br>• 确认 H-01~H-05 已在之前提交中修复<br>• 识别 8 个剩余问题 |
| `f7a51fe` | fix | **加固 Bridge JSON 解码 + PairCode 验证（M-01, M-07）**<br>• decodeJSONBody 替换 json.NewDecoder（未知字段拒绝）<br>• PairCode 配置字段 + subtle.ConstantTimeCompare<br>• 修复 ICP 指标预构建错误 |

### 5月21日（10 提交）

| 提交 | 类型 | 说明 |
|------|------|------|
| `1619e9d` | feat | **前端重设计 — 工业极简美学**<br>• 统一设计系统：CSS 自定义属性、Inter + JetBrains Mono 字体<br>• 紧凑头部、移除 emoji、简化模板<br>• style.css 从 ~2291 行缩减到 ~980 行 |
| `9972c0b` | fix | 提亮强调色为天蓝色（#0ea5e9） |
| `02aa8aa` | fix | **强调色改为护眼蓝绿色（#5b8c8e）**（降低饱和度，减少长时间使用眼疲劳） |
| `71e7c0a` | fix | **CSP 安全调度器操作按钮** — inline onclick 改为 data-* + 事件委托 |
| `a6c5680` | feat | **查询页面三层次视觉：Hero + 配置卡片 + 示例区域** |
| `3359fb4` | fix | **Cookie UI 改进 + Extension 状态统一 + 配额页面加速**<br>• Cookie: 移除冗余状态徽章、flex 布局、简化折叠按钮<br>• Extension 状态同步<br>• 配额页: GetQuota() 并发 goroutine 加速 |
| `3dc5f06` | fix | **配额错误消息截断 + 10s 获取超时**<br>• 长错误消息截断到第一行（最大 120 字符）<br>• 修复结果采集 goroutine 变量遮蔽 |
| `855c169` | feat | **账号页面（修改密码 + 当前用户）**<br>• POST /api/account/change-password（bcrypt）<br>• 配额页 FOFA/Shodan 卡片顺序修复（map 中间存储） |
| `542498a` | fix | **CDP Cookie 登录检测（Hunter/FOFA/Quake）**<br>• CDP 直读浏览器 Cookie 判断登录状态<br>• 前端 "browser_session" 状态显示 |
| `8c66e5f` | fix | **Extension Cookie 登录检测 + CDP URL/defer Bug 修复**<br>• CDP URL 前缀修复（https:// 补全）<br>• defer 移出 for 循环（超时累积修复）<br>• 提取 engineDomain()/judgeLoginByCookieNames() 消除重复<br>• Extension 增加 cookies 权限 + get_cookies action |

### 5月22日（3 提交）

| 提交 | 类型 | 说明 |
|------|------|------|
| `751ec5b` | docs | **深度审查报告（2026-05-22）**<br>• 架构/业务闭环/完整性/Bug 狩猎 4 维度<br>• 记录 1 个功能缺口 + 8 个 P1/P2 项目 |
| `b862905` | feat | **ICP 备案查询页面 + 设置页重组**<br>• /icp 独立页面，多类型搜索<br>• Cookie 管理/会话面板迁移到 /settings<br>• 新增 20 个测试（ICP 9 + Config 11） |
| `f24aa41` | feat | **ICP 定时任务（ST-21）**<br>• ICPQueryRunner + 上下文传播<br>• 两个默认模板（日度公司监控、周度域名扫描） |

### 5月23日（10 提交）

| 提交 | 类型 | 说明 |
|------|------|------|
| `c82380e` | feat | **ICP Phase 2 — 结果持久化 + 变更告警 + CSV 导入 + CLI 调度器**<br>• SQLite 结果持久化 + 时序对比<br>• IPC 变更检测告警（license/unit_name/update_record）<br>• CSV 关键词批量导入 Runner<br>• CLI 'scheduler' 子命令（list/run/create/enable/disable/delete/history） |
| `0819f07` | feat | **定时任务推送通知 + FOFA 自定义 API（S-1 至 S-13）**<br>• S-1: urlguard SSRF 保护包（14 测试）<br>• S-4~S-5: notify 包（钉钉/飞书/企微/Webhook + HMAC 签名）<br>• S-7~S-9: 调度器通知系统重写 + 前端多选通道 UI<br>• S-10~S-13: FOFA 双 URL 配置（API/Web 分离）<br>关键设计：两级开关、旧字段自动迁移、SSRF 集中校验 |
| `db42d6e` | fix | **连接 ICP 结果数据库 + 注册 ST-22 导入 Runner**<br>• Server.icpRepo 初始化（默认 ./data/icp_results.db）<br>• TaskICPImport（ST-22）注册<br>• E2E 预期任务类型 21→22 |
| `697a4a5` | docs | **notify+fofa 分支审查报告（2026-05-23）**<br>• 1 个 P0 回归（urlguard.Check 强制 DNS → 离线/CI 无法启动） |
| `72304e3` | fix | **urlguard.Check 与 DNS 解析解耦（P0 回归修复）**<br>• 拆分 checkIPLiteralPrivate（语法检查，配置验证时）和 checkHostLive（DNS + 私有检查，连接时） |
| `6662f78` | fix | **P0 urlguard DNS 解耦 + P1 FOFA 迁移 WARN 日志** |
| `5fa6496` | docs | 更新审查状态（P0+P1 修复后） |
| `6f285a7` | docs | **通知 API 文档 + RUNBOOK 排障章节**<br>• API.md §3.9 通知端点<br>• RUNBOOK.md §7 通知排障（6 步诊断） |
| `ee50be9` | fix | **P2 覆盖率提升 — urlguard (72.1%→86.2%) + notify (66.7%→90.4%)**<br>• urlguard: SafeDialer/SafeHTTPClient TLS 重定向/IPv6/空 URL<br>• notify: 全部通道 Getter/上下文取消/禁用通道/Reload/statusLabel |

### 5月24日（4 提交）

| 提交 | 类型 | 说明 |
|------|------|------|
| `5e1a215` | feat | **登录页重设计 — 暗色工业主题**<br>• 双面板布局（品牌 + 表单）<br>• 网格背景 + 发光动画<br>• 提交加载旋转器<br>• 响应式移动端适配 |
| `b8375f1` | docs | **综合深度审计报告（2026-05-24）**<br>• 4 维度：架构/业务完整性/项目就绪度(88%)/Bug 狩猎<br>• 4 P0 / 8 P1 / 8 P2 |
| `ce98eef` | docs | 优化整改计划 |
| `0fe1c76` | fix | **修复 DEEP_AUDIT 全部 P0/P1/P2 缺陷**<br>Phase 0 (P0): 9 个 unsafe atomic.Value 类型断言、os.Rename 错误吞掉、Bridge goroutine 泄漏、context 值类型断言<br>Phase 1 (P1): 多引擎结果去重、TamperCleanup 时间戳过滤、CacheWarmup SSRF、DNS 5s 超时、Session 撤销存储<br>Phase 2 (P2): WebSocket ID 加密随机、QueryStatus 深拷贝、截图写入错误检查、RWMutex 升级、retry 负延迟守卫、notifyWg.Wait |
| `48739cc` | fix | **审计反馈修复（Session 撤销/Tamper 清理/SSRF/图片格式）**<br>• TamperCleanupRunner 安全删除（全部过期才删）<br>• Session 撤销改用随机 session_id<br>• CacheWarmupRunner 使用 SafeHTTPClient<br>• 图片格式回归修复（PNG 默认分支）<br>• Monitor handler 顺序调整 |

### 5月25日（3 提交）

| 提交 | 类型 | 说明 |
|------|------|------|
| `6d6998d` | fix | **移除 setSessionCookie 中错误的登录时 session 撤销**（导致所有登录返回 401） |
| `55387d5` | feat | **通知通道管理页面（钉钉/飞书/企微/Webhook CRUD）**<br>• 设置页通知通道面板<br>• 通道保存/删除/测试 API<br>• 加密 Secret 存储 + env 解析<br>• CSP nonce 兼容：inline onclick → addEventListener |
| `f38a157` | fix | **飞书 Webhook/登录自动填充/Shodan 配额 Bug 修复**<br>• 飞书时间戳改为 string（签名校验要求）<br>• 密码输入框 autocomplete="new-password" 防止自动填充<br>• 通知测试结果可见性修复<br>• Shodan 配额引擎适配器热重载 |

### 5月26日（7 提交）

| 提交 | 类型 | 说明 |
|------|------|------|
| `27a088c` | fix | **飞书/Shodan 超时增至 30s + 飞书 Secret 占位符检测** |
| `a0b0e4d` | fix | 通知签名 + Shodan 配额显示修复 |
| `2c69aa0` | chore | **清理临时测试文件和 Agent Worktree** — 测试记录迁移到 docs/test_reports |
| `2555907` | docs | **重写 README.md**（基于当前项目状态） |
| `126a853` | docs | 修复 README Markdown 语法转义问题 |
| `a3b84ea` | chore | **移除审计/开发计划报告 Git 跟踪** — 更新为仅本地保留策略 |
| `b066ec4` | docs | **全局重命名 unimap-icp-hunter → unimap + 标记 v1.0.0 发布** |

---

## 第三周（5月27日 — 6月1日）：代码审查闭环 + 前端审计 + 浏览器降级 + 生产就绪

**主题**：全面代码审查修复（4阶段）、前端全量审计修复、浏览器查询降级全链路闭环、生产环境加固。

### 5月27日（3 提交）

| 提交 | 类型 | 说明 |
|------|------|------|
| `ad3721d` | fix | **综合代码审查修复（Phase 1-4）**<br>Phase 1 CRITICAL (6 项): 加密密钥环境变量、移除可预测 token fallback、RWMutex 配置保护、EngineAdapter.Search ctx 传播、GUI 安全类型断言、SSRF 保护<br>Phase 2 HIGH (4 项): X-Forwarded-Proto 检查、http.Server 超时、screenshot goroutine recover、CacheManager/LeakDetector/Registry 安全关闭<br>Phase 3 MEDIUM (7 项): chrome-extension CORS 限制、sanitizeBody 安全日志、统一 zap 日志、historyByTaskID 索引、错误链 %v→%w 修复(18处)、WebSocket ping/pong<br>Phase 4 LOW (8 项) + 死代码清理: 删除 4 个零引用包（objectpool/resourcepool/codequality/memory） |
| `769542e` | style | **gofmt 格式化全部 Go 文件**（修复 CI lint 检查的 233 个文件） |
| `f0b9cce` | test | **跳过无 Chrome 环境下的 CDP 依赖测试**（CI 兼容） |

### 5月28日（1 提交）

| 提交 | 类型 | 说明 |
|------|------|------|
| `980e918` | fix | **全量前端审计修复 + 篡改检测改进 + FOFA 字段降级**<br>前端 12 项修复: monitor/batch-screenshot 从零实现 JS 交互、引擎开关不可用修复、截图 enabled 持久化、缓存字段名匹配、无效复选框移除、详情/复制按钮类名修复、截图按钮 GET→POST、分页实现<br>CSP nonce 修复: inline script 改用 CSPNonce<br>篡改检测: 字段映射修正、历史记录 API、基线设置结果展示<br>FOFA: 全字段→基础字段自动降级（820001 错误）<br>浏览器查询: capture 模式改为 DOM 数据提取 |

### 5月29日（1 提交）

| 提交 | 类型 | 说明 |
|------|------|------|
| `828bf08` | feat | **浏览器查询降级全量闭环（阶段1-5 + 测试 + 前端 + 观测 + 运维）**<br>阶段1: ScreenshotRouter → Extension → CDP 降级链接通<br>阶段2: BrowserFallback 配置开关（默认全关）<br>阶段3: API 失败后可选 fallback + 资产 tag 标识<br>阶段4: 前端三色来源徽标（method-api/browser/browser-fallback）<br>阶段5: DOM 解析失败率 + login_required 计数器 + Grafana 5 面板<br>补充: 10 个 browserQueryProvider 测试、中文 toast 提示、Runbook §8 警告框、健康检查语义增强 |

### 5月30日（1 提交）

| 提交 | 类型 | 说明 |
|------|------|------|
| `3d6bf6a` | fix | **扩展图标入库 + urlguard 并发超时测试修复**<br>• 扩展图标（16/48/128.png）入库，修复 clone 后扩展无法加载<br>• urlguard AllowPrivate 测试改为 httptest 回环服务器（消除 :80 端口依赖）<br>go test -race ./... 全绿 |

### 5月31日（2 提交）

| 提交 | 类型 | 说明 |
|------|------|------|
| `69e928a` | docs | **补充批量截图示例私网地址 + 前端无错误处理的 P2 审查条目** |
| `8136620` | fix | **实施 PROJECT_REVIEW 全部 10 项修复**<br>P1: nginx upstream 端口 8080→8448 + WebSocket Upgrade 代理头、批量截图 500→200、停机顺序重排(HTTP drain→DB)、生产环境禁止默认 admin/admin<br>P2: Scheduler.Stop() 等待 cron 完成、WebSocket ping interval 泄漏修复、查询进度定向推送、/health/ready 增加 ICP DB/截图 Router/代理池检查<br>额外: nginx gzip 压缩 |

### 6月1日（2 提交）

| 提交 | 类型 | 说明 |
|------|------|------|
| `7813338` | fix | **合并截图页面到监控页 + 修复示例 URL 私网拦截**<br>• 示例 URL 去掉 test.example.org（RFC 2606 保留域名，触发 fail-closed 拦截）<br>• /batch-screenshot 301 → /monitor（monitor 是超集）<br>• 导航栏移除独立截图入口<br>• monitor.html 增加 resp.ok 错误检查 + Excel 导入支持 |
| `7bd32dc` | docs | **问题记录与整改计划** — 截图示例 URL/引擎报错展示/循环泄漏/ICP 多类型 |

---

## 主题聚合视图

### 🔐 安全加固（贯穿全月）

| 领域 | 关键提交 |
|------|----------|
| 认证系统 | `36011d3` Session Cookie、`5e1a215` 登录页重设计、`6d6998d` Session 撤销修复 |
| CSRF 保护 | `59f3a23` ~20 handler 覆盖、`36011d3` double-submit |
| SSRF 防护 | `0819f07` urlguard 包、`72304e3` DNS 解耦、`ad3721d` Phase 1 修复 |
| 密钥管理 | `ad3721d` NOTIFY_PEPPER 环境变量、`55387d5` 加密 Secret 存储 |
| CSP 策略 | `992ed71` 移除 unsafe-eval、`897173d` Scheduler nonce、`71e7c0a` 事件委托 |
| 并发安全 | `992ed71` atomic 修复、`0fe1c76` RWMutex + 深拷贝、`ad3721d` Phase 2 |

### 🌐 浏览器双模查询引擎

| 阶段 | 关键提交 | 内容 |
|------|----------|------|
| Phase 1 | `8271a6f` | Bridge 认证 + 门控 + SetMode |
| Phase 2 | `8be2de0` | 双模抽象 + Collect 能力 + UI 选择器 |
| Phase 3 | `25e59b5` | DOM 提取主管道 + WebOnly 适配器 |
| Phase 4 | `f3e9c4c` + `137a027` | 前端统一 + Structured 采集 |
| 修复 | `738d5e1` | 6 项修复（UQL/合并/进度/失败检测/登录/字段） |
| Extension | `2044053` + `4584e9d` | capture.js v0.2.0 升级（卡片选择器/登录墙/SPA） |
| 降级闭环 | `828bf08` | 阶段1-5 全量闭环（降级链/开关/徽标/观测/Runbook） |

### 📋 ICP 备案查询全链路

| 阶段 | 关键提交 | 内容 |
|------|----------|------|
| 页面 | `b862905` | /icp 独立页面 + 设置重组 + 20 测试 |
| 定时任务 | `f24aa41` | ST-21 ICPQueryRunner + 默认模板 |
| 持久化 | `c82380e` | SQLite 存储 + 变更告警 + CSV 导入 + CLI |
| 修复 | `db42d6e` | icpRepo 初始化 + ST-22 注册 |

### 🔔 通知推送系统

| 阶段 | 关键提交 | 内容 |
|------|----------|------|
| 基础包 | `0819f07` | notify 包（26 测试）+ urlguard SSRF 保护 |
| 通道管理 | `55387d5` | CRUD UI + 加密存储 + 测试 API |
| Bug 修复 | `f38a157` + `27a088c` | 飞书签名/超时/占位符检测 |
| 覆盖率 | `ee50be9` | notify 66.7% → 90.4% |

### 🎨 前端重设计

| 阶段 | 关键提交 | 内容 |
|------|----------|------|
| 工业极简 | `1619e9d` | CSS 变量 + Inter 字体 + 980 行 |
| 色彩迭代 | `9972c0b` → `02aa8aa` | 天蓝 → 护眼蓝绿 |
| Hero 布局 | `a6c5680` | 三层次视觉层次 |
| 暗色登录 | `5e1a215` | 双面板 + 网格动画 |

### 🔧 代码审查闭环

| 审查批次 | 发现 | 修复提交 |
|----------|------|----------|
| 第3轮安全审查 | 13 项 C/H/M | `992ed71` |
| 4月29日深度审查 | 11 项剩余 | `8446404` |
| 5月19日综合审查 | 6 项 | `0406532` |
| 5月22日深度审查 | 1 GAP + 8 P1/P2 | `0fe1c76` + `48739cc` |
| Phase 1-4 全量 | 32 项 + 4 死代码包 | `ad3721d` |
| PROJECT_REVIEW | 10 项 | `8136620` |

### 📊 测试与覆盖率

| 领域 | 关键提交 | 结果 |
|------|----------|------|
| urlguard | `ee50be9` | 72.1% → 86.2%（14→新测试） |
| notify | `ee50be9` | 66.7% → 90.4% |
| browserQueryProvider | `828bf08` | 10 个 table-driven 测试 |
| CDP 跳过 | `f0b9cce` | CI 环境兼容 |
| urlguard 并发 | `3d6bf6a` | 消除 :80 端口依赖，-race 全绿 |

---

## 关键里程碑

| 日期 | 里程碑 |
|------|--------|
| 5月6日 | 4月29日深度代码审查全部修复完成 |
| 5月7日 | 登录认证系统上线 + 浏览器双模查询引擎 Phase 2-4 完成 |
| 5月9日 | 浏览器查询 6 项修复全闭环 + 第3轮安全审查通过 |
| 5月21日 | 前端工业极简重设计完成 |
| 5月22日 | ICP 备案查询页面 + 定时任务上线 |
| 5月23日 | ICP Phase 2 持久化 + 通知推送系统全链路完成 |
| 5月24日 | DEEP_AUDIT 全部 P0/P1/P2 修复（20 项） |
| 5月26日 | **v1.0.0 正式发布**（README 重写、全局命名统一） |
| 5月27日 | Phase 1-4 综合代码审查（32 项修复 + 4 死代码包清理） |
| 5月28日 | 前端全量审计修复（12 项交互修复） |
| 5月29日 | 浏览器查询降级全量闭环（5 阶段 + 测试 + 观测 + 运维） |
| 5月31日 | PROJECT_REVIEW 全部 10 项修复 |
| 6月1日 | 截图页面合并 + 问题记录与整改计划 |

---

## 总结

2026年5月是 UniMap 项目从功能开发走向 **生产就绪** 的关键月份。77 个提交覆盖了以下核心工作：

1. **安全体系**从零搭建：登录认证、Session 管理、CSRF/SSRF 防护、CSP 策略、密钥管理全面到位
2. **浏览器双模查询引擎**从概念到全量闭环：CDP/Extension 双模抽象、DOM 提取、降级链、观测指标
3. **ICP 备案查询**全链路：页面 → 定时任务 → 持久化 → 变更告警 → CLI
4. **推送通知系统**：钉钉/飞书/企微/Webhook 四通道 + HMAC 签名 + 加密存储
5. **代码质量**：5 轮深度审查、32+ 项安全修复、4 个死代码包清理、urlguard/notify 覆盖率 85%+
6. **前端重设计**：工业极简美学、暗色登录页、CSP 兼容
7. **v1.0.0 正式发布**，go test -race 全链路绿色通过

---

> 文档生成时间：2026-06-01
> 生成方式：`git log --since="2026-05-01" --until="2026-06-02" --no-merges`
