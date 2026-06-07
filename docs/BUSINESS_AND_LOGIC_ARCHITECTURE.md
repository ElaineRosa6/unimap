# UniMap 业务架构与逻辑架构文档

> 版本：2026-06-05 | 分支：release/major-upgrade-vNEXT

---

## 一、系统定位

UniMap 是一款**多引擎网络空间资产查询与网页监控平台**，统一对接 FOFA、Hunter、ZoomEye、Quake、Shodan 五大搜索引擎，提供资产查询、截图监控、篡改检测、定时任务、分布式节点、告警通知等能力。支持 Web / CLI / GUI / Chrome Extension 四种前端形态。

---

## 二、业务架构

### 2.1 业务域全景

```
┌─────────────────────────────────────────────────────────────────┐
│                        用户入口层                                │
│   Web UI (8448)   CLI (子命令/直连)   GUI (Fyne)   Extension    │
└────────┬──────────────┬───────────────┬────────────┬────────────┘
         │              │               │            │
┌────────▼──────────────▼───────────────▼────────────▼────────────┐
│                     业务编排层 (Service)                         │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────────┐   │
│  │ QueryApp │ │ScreenApp │ │TamperApp │ │  MonitorApp      │   │
│  │ Service  │ │ Service  │ │ Service  │ │  Service         │   │
│  └────┬─────┘ └────┬─────┘ └────┬─────┘ └────┬─────────────┘   │
│       │            │            │             │                  │
│  ┌────▼────────────▼────────────▼─────────────▼──────────────┐  │
│  │              UnifiedService (统一服务入口)                  │  │
│  └────────────────────────┬───────────────────────────────────┘  │
└───────────────────────────┼──────────────────────────────────────┘
                            │
┌───────────────────────────▼──────────────────────────────────────┐
│                       核心能力层                                  │
│                                                                   │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌──────────┐ ┌──────────┐ │
│  │ Adapter │ │   UQL   │ │ Screenshot│ │  Tamper  │ │  Plugin  │ │
│  │ Engine  │ │ Parser  │ │ CDP/Ext │ │ Detector │ │ Manager  │ │
│  │ Orch.   │ │ Merger  │ │ Router  │ │ Baseline │ │ Hooks    │ │
│  └────┬────┘ └────┬────┘ └────┬────┘ └────┬─────┘ └────┬─────┘ │
│       │           │           │            │            │        │
│  ┌────▼───┐  ┌────▼───┐  ┌───▼────┐  ┌───▼─────┐  ┌───▼─────┐ │
│  │ FOFA   │  │ Cache  │  │ Bridge │  │ Database│  │Pipeline │ │
│  │ Hunter │  │ Redis/ │  │Service │  │  (SQLite│  │         │ │
│  │ ZoomEye│  │ Memory │  │        │  │  /File) │  │         │ │
│  │ Quake  │  └────────┘  └────────┘  └─────────┘  └─────────┘ │
│  │ Shodan │                                                      │
│  │ ICP    │                                                      │
│  └────────┘                                                      │
└──────────────────────────────────────────────────────────────────┘
                            │
┌───────────────────────────▼──────────────────────────────────────┐
│                      基础设施层                                    │
│                                                                   │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐           │
│  │Scheduler │ │Distributed│ │ Alerting │ │  Notify  │           │
│  │ 22 Runner│ │ Node/Task │ │ Threshold│ │ 6 Channel│           │
│  │ Cron     │ │ Queue     │ │ Silence  │ │ Registry │           │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘           │
│                                                                   │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐           │
│  │  Logger  │ │  Metrics │ │ Monitor  │ │  Backup  │           │
│  │ zap+async│ │Prometheus│ │ Resource │ │ tar.gz   │           │
│  │ LevelMgr │ │ 20+指标  │ │ LeakDet  │ │ AutoClean│           │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘           │
│                                                                   │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐                         │
│  │   Auth   │ │  Config  │ │  Proxy   │                         │
│  │ RBAC     │ │ YAML+Env │ │  Pool    │                         │
│  │ Session  │ │ HotReload│ │ RoundRob │                         │
│  │ API Key  │ │ Encrypt  │ │ Cooldown │                         │
│  └──────────┘ └──────────┘ └──────────┘                         │
└──────────────────────────────────────────────────────────────────┘
```

### 2.2 核心业务流程

#### 2.2.1 资产查询主流程

```
用户输入 UQL
    │
    ▼
UQL Parser ──→ AST (抽象语法树)
    │
    ▼
EngineOrchestrator
    │
    ├──→ FOFA 适配器 ──→ Translate(AST) ──→ FOFA 查询语法 ──→ API 请求
    ├──→ Hunter 适配器 ──→ Translate(AST) ──→ Hunter 查询语法 ──→ API 请求
    ├──→ ZoomEye 适配器 ──→ Translate(AST) ──→ ZoomEye 查询语法 ──→ API 请求
    ├──→ Quake 适配器 ──→ Translate(AST) ──→ Quake 查询语法 ──→ API 请求
    └──→ Shodan 适配器 ──→ Translate(AST) ──→ Shodan 查询语法 ──→ API 请求
                                    (并行执行，带熔断器+重试+QPS控制)
    │
    ▼
各引擎 EngineResult
    │
    ▼
Normalize() ──→ []UnifiedAsset (统一资产结构)
    │
    ▼
ResultMerger ──→ 按 IP:Port 去重，引擎优先级合并字段
    │
    ▼
Cache 缓存 ──→ 返回 QueryResponse
    │
    ▼ (可选)
Browser Query ──→ 打开结果页 / 采集DOM / 自动截图
```

#### 2.2.2 截图双模式高可用

```
截图请求
    │
    ▼
ScreenshotRouter
    │
    ├── CDP 模式 (健康?) ──→ chromedp 驱动 Chrome ──→ 截图
    │       │ 失败
    │       ▼
    │   降级到 Extension 模式
    │
    └── Extension 模式 (健康?) ──→ BridgeService ──→ Chrome Extension ──→ 截图
            │ 失败
            ▼
        降级到 CDP 模式

HealthChecker: 每 30 秒探测 CDP (/json/version) 和 Bridge 状态
自动降级，无需人工干预
```

#### 2.2.3 篡改检测流程

```
URL 列表
    │
    ▼
TamperDetector.BatchCheckTampering()
    │
    ├──→ 渲染页面 (CDP/Extension)
    ├──→ 计算分段哈希 (head/body/nav/main/footer/scripts/...)
    ├──→ 与基线对比
    │       │
    │       ├── 相同 → 正常
    │       └── 不同 → 标记篡改
    │               │
    │               ├── 精确模式: 任何变化都告警
    │               ├── 宽松模式: 忽略非关键区域
    │               └── 安全模式: 仅检测恶意脚本/可疑内容
    │
    ▼
检测到篡改 ──→ Alerting ──→ Notify (飞书/钉钉/企微/Webhook)
```

#### 2.2.4 定时任务调度

```
Scheduler (robfig/cron)
    │
    ├── 22 种 Runner 按 cron 表达式触发
    │
    ├── 高优先级:
    │   ├── QueryRunner         定时资产查询
    │   ├── SearchScreenshot    搜索引擎截图
    │   ├── BatchScreenshot     批量URL截图
    │   ├── TamperCheck         篡改检测
    │   ├── URLReachability     URL可达性
    │   ├── CookieVerify        Cookie验证
    │   ├── LoginStatusCheck    登录状态
    │   └── DistributedSubmit   分布式任务提交
    │
    ├── 中优先级:
    │   ├── Export              数据导出
    │   ├── PortScan            端口扫描
    │   ├── ScreenshotCleanup   截图清理
    │   ├── TamperCleanup       篡改记录清理
    │   ├── QuotaMonitor        配额监控
    │   ├── AlertSummary        告警汇总
    │   ├── BaselineRefresh     基线刷新
    │   └── URLImport           URL导入
    │
    └── 低优先级:
        ├── PluginHealth        插件健康
        ├── BridgeHealthCheck   Bridge健康
        ├── AlertSilence        告警静默
        ├── URLHealthChecker    URL健康
        ├── ICPQuery            ICP备案查询
        └── ICPImport           ICP关键词导入

高级特性:
    - 任务依赖链 (循环依赖检测)
    - 执行窗口 (时区支持, 工作日限制)
    - 通知配置 (channel IDs 引用)
    - 持久化 (SQLite)
```

### 2.3 业务角色与权限

| 角色 | 权限范围 | 典型场景 |
|------|---------|---------|
| **admin** | 全部权限 | 系统管理员 |
| **operator** | 查询/截图/篡改检测/任务管理 (无删除/节点管理/配置写入) | 安全运营 |
| **readonly** | 只读查看 | 审计/汇报 |
| **node** | API 执行 + 节点注册 | 分布式节点 |

API Key 体系：32 字节随机密钥，SHA-256 哈希持久化，支持过期/撤销/权限粒度控制。

---

## 三、逻辑架构

### 3.1 分层架构

```
┌─────────────────────────────────────────────────────────────────┐
│  Layer 1: 入口层 (cmd/)                                         │
│  unimap-web / unimap-cli / unimap-gui / Extension               │
│  职责: 参数解析、初始化、启动入口                                  │
└────────────────────────┬────────────────────────────────────────┘
                         │
┌────────────────────────▼────────────────────────────────────────┐
│  Layer 2: HTTP 层 (web/)                                        │
│  路由注册、中间件链、请求/响应处理、模板渲染                        │
│                                                                  │
│  中间件链 (从外到内):                                              │
│  audit → metrics → auth → CORS → sizeLimit → requestID →        │
│  security → mux(路由分发) → [rateLimit + optionalAPIKey]          │
└────────────────────────┬────────────────────────────────────────┘
                         │
┌────────────────────────▼────────────────────────────────────────┐
│  Layer 3: 应用层 (service/)                                      │
│  QueryAppService / ScreenshotAppService / TamperAppService /     │
│  MonitorAppService                                                │
│  职责: 业务编排、跨模块协调、浏览器降级                             │
└────────────────────────┬────────────────────────────────────────┘
                         │
┌────────────────────────▼────────────────────────────────────────┐
│  Layer 4: 核心层 (internal/)                                     │
│                                                                  │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐           │
│  │ adapter/ │ │ core/    │ │screenshot/│ │ tamper/  │           │
│  │ 5引擎+ICP│ │ UQL解析  │ │ CDP/Ext  │ │ 篡改检测 │           │
│  │ 熔断+重试│ │ 结果归并 │ │ Router   │ │ 基线管理 │           │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘           │
│                                                                  │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐           │
│  │scheduler/│ │distribut/│ │ alerting/│ │ notify/  │           │
│  │ 22 Runner│ │ 节点管理 │ │ 阈值告警 │ │ 6渠道    │           │
│  │ Cron调度 │ │ 任务队列 │ │ 静默窗口 │ │ 热重载   │           │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘           │
│                                                                  │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐           │
│  │ plugin/  │ │ auth/    │ │ config/  │ │ exporter/│           │
│  │ 4种插件  │ │ RBAC     │ │ YAML热更 │ │ JSON/Excel│          │
│  │ 钩子机制 │ │ Session  │ │ 环境变量 │ │ 篡改导出 │           │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘           │
└────────────────────────┬────────────────────────────────────────┘
                         │
┌────────────────────────▼────────────────────────────────────────┐
│  Layer 5: 基础设施层                                              │
│                                                                  │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐           │
│  │ logger/  │ │ metrics/ │ │monitoring/│ │ backup/  │           │
│  │ zap      │ │Prometheus│ │ 资源监控 │ │ tar.gz   │           │
│  │ 异步写入 │ │ 20+指标  │ │ 泄漏检测 │ │ 自动清理 │           │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘           │
│                                                                  │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐                         │
│  │ proxypool/│ │ error/  │ │ model/   │                         │
│  │ 轮询代理 │ │ 统一错误 │ │ 数据模型 │                         │
│  │ 冷却恢复 │ │ 6类错误码│ │ 接口定义 │                         │
│  └──────────┘ └──────────┘ └──────────┘                         │
└─────────────────────────────────────────────────────────────────┘
```

### 3.2 核心数据模型

#### UnifiedAsset (统一资产结构)

贯穿整个系统的核心数据结构，从引擎适配器产出，经缓存、归并、导出全链路流转。

```go
type UnifiedAsset struct {
    IP          string            // IP 地址
    Port        int               // 端口号
    Protocol    string            // 协议 (http/https/tcp/...)
    Host        string            // 主机名
    URL         string            // 完整 URL
    Title       string            // 页面标题
    BodySnippet string            // 正文摘要
    Server      string            // 服务器类型
    Headers     map[string]string // HTTP 响应头
    StatusCode  int               // HTTP 状态码
    CountryCode string            // 国家代码
    Region      string            // 省份/州
    City        string            // 城市
    ASN         int               // 自治系统号
    Org         string            // 组织
    ISP         string            // 运营商
    Source      string            // 来源引擎
    Extra       map[string]string // 扩展字段
}
```

#### EngineAdapter 接口

```go
type EngineAdapter interface {
    Name() string
    Translate(ast *UQLAST) (string, error)                    // UQL → 引擎查询语法
    Search(ctx context.Context, query string, page, pageSize int) (*EngineResult, error)
    Normalize(raw *EngineResult) ([]UnifiedAsset, error)      // 原始结果 → 统一资产
    GetQuota() (*QuotaInfo, error)                            // 引擎配额
    IsWebOnly() bool                                          // 是否仅Web模式
}
```

#### UQL 语法支持

```
操作符:  =  !=  ~=  IN  CONTAINS  >  <
逻辑符:  && / AND   || / OR
分组:    ( )
字段:    country  port  protocol  host  title  server  ip  ...

示例:
  country="CN" && port="80"
  (title="登录" || title="login") && protocol="https"
  port IN ["80","443","8080"]
```

### 3.3 模块间依赖关系

```
Config ────────────────────────────────────────────────┐
  │                                                     │
  ├─→ Engines ──→ EngineAdapter ──→ UnifiedAsset        │
  │                    │                   │            │
  │                    │                   ├─→ Exporter  │
  │                    │                   └─→ Merger   │
  │                    │                                 │
  │                    └─→ EngineResult ──→ Cache        │
  │                                                     │
  ├─→ Web.Auth ──→ UserDB (SQLite)                      │
  │            ──→ APIKeyManager (SHA-256)               │
  │            ──→ AuthMiddleware                        │
  │            ──→ PermissionManager (RBAC)              │
  │                                                     │
  ├─→ Screenshot ──→ metrics                            │
  │              ──→ Bridge (Extension)                  │
  │              ──→ Router (CDP/Extension HA)           │
  │                                                     │
  ├─→ Distributed ──→ NodeRegistry                      │
  │               ──→ TaskQueue                          │
  │               ──→ FailoverStrategy                   │
  │                                                     │
  ├─→ Scheduler ──→ 22 Runners                          │
  │              ──→ ExecutionHistory (SQLite)            │
  │              ──→ TaskTemplates                       │
  │                                                     │
  ├─→ Alerting ──→ Thresholds                           │
  │             ──→ SilenceWindows                      │
  │             ──→ AlertRecords                         │
  │                                                     │
  ├─→ Notify ──→ Registry (6 channel types)             │
  │           ──→ HotReload                             │
  │                                                     │
  ├─→ Cache ──→ Memory / Redis                          │
  │                                                     │
  ├─→ Logger ──→ LevelManager (动态级别)                 │
  │           ──→ ErrorAlertHook (滑动窗口)              │
  │           ──→ AsyncWriter                            │
  │                                                     │
  ├─→ Metrics ──→ Prometheus (20+ 指标族)                │
  │                                                     │
  ├─→ Monitoring ──→ ResourceMonitor                    │
  │               ──→ LeakDetector                      │
  │                                                     │
  └─→ ProxyPool ──→ RoundRobin + Cooldown               │
```

### 3.4 认证与安全架构

#### 3.4.1 认证方式

| 方式 | 适用场景 | 实现 |
|------|---------|------|
| Session Cookie | 浏览器用户 | AES-256-GCM 加密，24h 有效期，含 sessionID + userID + adminToken |
| Admin Token | API/CLI | `web.auth.admin_token` 常量时间比较 |
| Node Token | 分布式节点 | `distributed.node_auth_tokens` 按 node_id 匹配 |
| API Key | 第三方集成 | 32 字节随机密钥，SHA-256 哈希，支持过期/撤销/权限粒度 |

#### 3.4.2 安全防护

| 防护层 | 措施 |
|--------|------|
| **传输** | HTTPS, HSTS, Strict-Transport-Security |
| **响应头** | X-Frame-Options: DENY, X-Content-Type-Options: nosniff, CSP (nonce-based), Referrer-Policy, Permissions-Policy |
| **CORS** | 严格 Origin 白名单, Bridge 路径例外 |
| **CSRF** | SameSite=Strict CSRF cookie, 登录页验证 |
| **限流** | 滑动窗口 60次/分钟, 登录防暴力 5次/15分钟/IP |
| **输入验证** | SSRF 防护 (私有IP+DNS解析后校验), 请求体 10MB 限制, 文件名消毒 |
| **存储** | 密码 bcrypt 哈希, API Key 仅存哈希, 通知密钥加密存储, 配置支持环境变量 |
| **会话** | AES-GCM 加密 cookie, 服务端会话撤销, SameSite=Lax |
| **Bridge** | loopback-only 配对, HMAC-SHA256 回调签名, nonce 防重放, 时间戳偏差校验 |
| **错误处理** | sanitizeError 去除堆栈跟踪和内部路径 |

### 3.5 可观测性架构

#### Prometheus 指标 (20+ 指标族)

| 类别 | 指标前缀 | 标签维度 |
|------|---------|---------|
| HTTP 请求 | `unimap_http_*` | path, method, status |
| 限流 | `unimap_rate_limit_*` | path |
| 统一查询 | `unimap_query_*` | status |
| 引擎查询 | `unimap_engine_query_*` | engine, status |
| 缓存 | `unimap_cache_*` | backend, result |
| 引擎错误 | `unimap_engine_errors_*` | engine |
| 篡改检测 | `unimap_tamper_*` | status |
| 截图 | `unimap_screenshot_*` | type, status |
| WebSocket | `unimap_websocket_*` | direction |
| 资源 | `unimap_goroutines_count`, `unimap_memory_*` | — |
| 批量操作 | `unimap_batch_*` | type |
| Bridge | `unimap_screenshot_bridge_*` | engine, status |
| 截图路由 | `unimap_screenshot_mode_*`, `unimap_screenshot_health_*` | mode, result |
| 调度器 | `unimap_scheduler_*` | task_type, status |
| ICP | `unimap_icp_*` | type, status |
| 浏览器降级 | `unimap_browser_fallback_*` | engine, trigger/reason |

#### 日志系统

- **引擎**: zap (异步写入, 动态级别)
- **结构化字段**: app, env, version, hostname, request_id
- **轮转**: lumberjack, 可配置 MaxSize/MaxBackups/MaxAge
- **告警钩子**: 滑动窗口 ERROR 计数，阈值触发回调

#### 告警阈值

| 指标 | 警告 | 严重 |
|------|------|------|
| 查询 P95 延迟 | > 30s | > 60s |
| 缓存命中率 | < 50% | < 20% |
| 截图成功率 | < 90% | < 70% |
| 节点在线率 | < 80% | < 50% |
| Goroutine 数 | > 1000 | > 5000 |
| 磁盘使用 | > 80% | > 90% |

---

## 四、API 架构

### 4.1 API 版本策略

- **规范路径**: `/api/v1/...` (推荐)
- **旧版路径**: `/api/...` (带 `Deprecation: true` 和 `Sunset: 2026-09-01` 响应头)
- **双注册**: `addAPIRoute` 同时注册两个版本

### 4.2 API 端点分组

| 分组 | 端点数 | 限流 | 说明 |
|------|--------|------|------|
| 查询 | 3 | 是 | UQL 查询、状态轮询 |
| 截图 | 17 | 部分 | 单张/批量/搜索引擎/目标/Bridge/Router |
| Cookie | 4 | 否 | 保存/验证/导入/状态检测 |
| CDP | 2 | 否 | 连接状态/启动连接 |
| 定时任务 | 9 | 否 | CRUD + 执行/启用/禁用/历史 |
| 通知 | 5 | 否 | 渠道 CRUD + 测试 + 重载 |
| 篡改检测 | 6 | 部分 | 检测/基线/历史 |
| 分布式节点 | 12 | 否 | 注册/心跳/注销/任务队列/管理 |
| ICP 备案 | 5 | 是 | 查询/健康/历史/对比 |
| 配置 | 2 | 否 | 读取/保存 (section级别) |
| 用户 | 6 | 否 | 注册/CRUD/密码修改 |
| 账号 | 2 | 否 | 密码修改/Admin Token |
| 监控 | 3 | 是 | URL导入/可达性/端口扫描 |
| 备份 | 2 | 否 | 创建/列表 |
| 健康 | 3 | 否 | ready/live/health |
| WebSocket | 1 | 否 | 实时查询 |
| 指标 | 1 | 否 | Prometheus |

**总计**: 176 条路由

### 4.3 WebSocket 协议

| type | 方向 | 说明 |
|------|------|------|
| `ping` / `pong` | 双向 | 应用级心跳 |
| `query` | C→S | 发起查询 |
| `query_start` | S→C | 查询开始确认 |
| `progress_update` | S→C | 进度 (0-100) |
| `query_complete` | S→C | 查询完成 |
| `query_error` | S→C | 查询错误 |

心跳: 服务端 30s Ping, 60s Pong 超时断开。查询 60s 超时, 5 分钟自动清理。

---

## 五、数据存储架构

| 存储 | 用途 | 技术 |
|------|------|------|
| 配置文件 | 系统配置 | YAML + 环境变量解析 + 密钥加密 |
| 用户数据库 | 用户/角色 | SQLite (WAL 模式) |
| 任务持久化 | 定时任务/执行历史 | SQLite |
| 节点快照 | 分布式节点状态 | JSON 文件 |
| API Key | 密钥哈希 | JSON 文件 |
| 截图文件 | 截图产出 | 文件系统 (按批次目录) |
| 篡改基线 | 页面哈希 | 文件系统 |
| 缓存 | 查询结果 | 内存 / Redis (可配置) |
| ICP 数据库 | 备案查询结果 | SQLite (sidecar) |
| 备份 | 配置/数据备份 | tar.gz 文件 |

---

## 六、部署架构

### 6.1 单机部署

```
┌─────────────────────────────────────┐
│  unimap-web (:8448)                 │
│  ├── Web UI                         │
│  ├── API Server                     │
│  ├── Scheduler (22 Runners)         │
│  ├── Screenshot (CDP/Extension)     │
│  ├── Tamper Detector                │
│  └── All internal services          │
│                                     │
│  Chrome (CDP :9222 or Extension)    │
│  SQLite (data/)                     │
│  Config (configs/config.yaml)       │
└─────────────────────────────────────┘
```

### 6.2 分布式部署

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  主节点      │     │  工作节点 A  │     │  工作节点 B  │
│  unimap-web  │     │  unimap-node │     │  unimap-node │
│  ├── Web UI  │◄────│  ├── 心跳    │     │  ├── 心跳    │
│  ├── API     │     │  ├── 领取任务│     │  ├── 领取任务│
│  ├── Scheduler│    │  └── 提交结果│     │  └── 提交结果│
│  └── TaskQueue│    └──────────────┘     └──────────────┘
└──────────────┘
         │
         ▼
   故障转移策略: health_based / load_balanced / priority_based
```

### 6.3 Docker 部署

- 基础镜像: alpine:3.21
- 非 root 用户
- 环境变量覆盖配置

---

## 七、技术栈总览

| 类别 | 技术 |
|------|------|
| 语言 | Go 1.26 |
| Web | net/http + gorilla/websocket + go-resty |
| GUI | Fyne v2 |
| CLI | Cobra |
| 浏览器自动化 | chromedp (CDP protocol) |
| 定时任务 | robfig/cron/v3 |
| 缓存 | 内存 + Redis (go-redis/v9) |
| 存储 | SQLite, YAML, JSON |
| HTML 解析 | goquery + bluemonday |
| 导出 | excelize (Excel) |
| 监控 | Prometheus client_golang |
| 日志 | zap (异步写入, 动态级别) |
| CI | GitHub Actions (test + lint + race + security + Docker push) |

---

## 八、扩展点

| 扩展点 | 机制 | 说明 |
|--------|------|------|
| 搜索引擎 | EngineAdapter 接口 | 新增引擎只需实现 5 个方法 |
| 数据处理 | ProcessorPlugin + Pipeline | 按优先级串联处理 |
| 导出格式 | ExporterPlugin | 新增导出格式 |
| 通知渠道 | NotifyChannel 接口 | 新增通知渠道 |
| 定时任务 | Runner 注册 | 新增 Runner 类型 |
| 查询钩子 | before_query / after_query / query_error | 插件可拦截查询流程 |
| 生命周期钩子 | before_load/after_load/... (8种) | 插件生命周期管理 |
