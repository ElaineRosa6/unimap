# 全量实施计划 — 空间搜索引擎 + 遗留问题

> **创建日期**: 2026-06-07
> **状态**: 📋 计划阶段
> **基准文档**: `SEARCH_ENGINE_SYNTAX_REFERENCE.md`（10+ 引擎完整语法）、`SEARCH_ENGINE_SYNTAX.md`（UQL→引擎翻译基准）
> **前置依赖**: commit `0e3fcc3`（5 引擎语法修正闭环）
> **来源**: CLAUDE.md 已知待修复事项、code review 发现、memory 遗留缺陷、三层采集架构设计评审

---

## 目录

- [〇、全局待解决问题清单](#〇全局待解决问题清单)
- [一、阶段一：核查现有 5 引擎适配器](#一阶段一核查现有-5-引擎适配器)
- [二、阶段二：新增搜索引擎](#二阶段二新增搜索引擎)
- [三、阶段三：三层采集架构](#三阶段三层采集架构)
- [四、每个引擎实施步骤](#四每个引擎实施步骤)
- [五、字段映射速查](#五字段映射速查)
- [六、风险与依赖](#六风险与依赖)

---

## 〇、全局待解决问题清单

> 汇总自 CLAUDE.md 已知待修复事项、adapter code review、memory 遗留缺陷、三层采集架构设计评审。

### 0.1 安全与合规（紧急）

| # | 问题 | 来源 | 状态 | 行动 |
|---|------|------|------|------|
| SEC-1 | `.claude/settings.local.json` 含 admin token `1AggpIRXaHIQnH73SawdMLDfB8RnIy3X`，是 git 已跟踪文件 | memory 2026-06-07 | ⚠️ 未处理 | 加入 `.gitignore`；若曾提交历史需评估是否轮换 token |
| SEC-2 | API 版本化旧路径 shim sundown **2026-09-01** | memory 2026-05-31 | ⏰ 待处理 | 届时移除无 `/api/v1` 前缀的旧路由注册（`server.go` 73 条 shim） |

### 0.2 代码质量 — 技术债务（CLAUDE.md 记录）

| # | 问题 | 来源 | 行动 |
|---|------|------|------|
| TD-1 | 10 个文件超 800 行（最大 `monitor_native.go` 2150 行） | CLAUDE.md Medium | 按功能拆分模块，优先拆 `monitor_native.go` |
| TD-2 | 34 个函数超 50 行（最大 `createMonitorTab` 390 行） | CLAUDE.md Medium | 提取子函数，遵循 <50 行规范 |
| TD-3 | 错误消息大写 23 处（多数为缩写词可接受） | CLAUDE.md Low | 逐条审查，非缩写词改为小写 |
| TD-4 | `map[string]interface{}` 强类型（插件接口广泛使用） | CLAUDE.md Low | 渐进重构，定义 `PluginResult` 等结构体 |

### 0.3 适配器 Code Review 遗留（2026-06-07 review）

| # | 问题 | 级别 | 来源 | 行动 |
|---|------|------|------|------|
| CR-1 | Quake `>` 和 `>=` 输出相同 `[N TO *]`（无排他下界） | MEDIUM | code review M-3 | 文档已记录为已知限制，Quake 语法本身不支持排他区间 |
| CR-2 | Shodan OR 降级为 AND（结果集更小） | MEDIUM | code review M-2 | 已加 `logger.Warnf`；长期考虑在 UI 层提示用户"Shodan 不支持跨字段 OR" |
| CR-3 | ZoomEye/Hunter 比较操作符值加引号（`port>"80"` 而非 `port>80`） | MEDIUM | code review M-5/M-6 | 需对照 ZoomEye v2 / Hunter 官方文档确认数值比较是否需引号 |
| CR-4 | Shodan `header` 和 `body` 都映射到 `http.html` | LOW | code review L-2 | Shodan 无独立 header filter，已接受折中；文档记录 |
| CR-5 | ZoomEye `url` 映射到 `site`（旧映射未更新） | LOW | code review L-3 | 需确认 ZoomEye v2 是否有 `url` 独立字段 |

### 0.4 采集架构 — 三层采集设计评审遗留（2026-06-06）

> 完整设计见 `docs/THREE_LAYER_COLLECTION_ARCHITECTURE.md`，实施计划见 `docs/THREE_LAYER_IMPLEMENTATION_PLAN.md`

| # | 问题 | 级别 | 要点 | 行动 |
|---|------|------|------|------|
| ARC-1 | L2 hook 在 ISOLATED world，拦不到 fetch/XHR | 🔴 C-1 | MV3 content script 默认 ISOLATED world，不共享 JS 全局 | Phase 1 前置：`world: MAIN` 注入 + postMessage 桥（见 ARC-2） |
| ARC-2 | MAIN world 无 `chrome.runtime` | 🔴 C-2 | 拦截(MAIN) 与 回传(ISOLATED) 互斥 | 两段式注入：MAIN world 拦截 → postMessage → ISOLATED world 回传 |
| ARC-3 | L1 与 L2 同源冗余非正交 | 🟠 H-1 | 都拦同一份 API 响应，改端点/加密同时失效 | 建议先做 L1，L2 暂缓 |
| ARC-4 | API 端点为猜测需抓包验证 | 🟠 H-2 | Shodan 列的是官方 API 非网页端点 | Phase -1：抓包 spike 验证各引擎网页端点 |
| ARC-5 | 加密响应体无应对 | 🟠 H-3 | 强反爬引擎 L1/L2 归零退回 DOM | 2026-06-07 更新：5 引擎 Extension 采集已全部打通，H-3 风险评估不变 |
| ARC-6 | collection 代码塞进 `internal/screenshot/` 包 | 🟡 M-1 | 职责膨胀 | Phase 0：迁移到 `internal/collection/` |
| ARC-7 | `<all_urls>` 全网注入 | 🟡 M-2 | 与"降低检测"矛盾 | 收窄至 5 引擎域名（+ 新增引擎域名） |
| ARC-8 | L1/L3 独立 context 请求翻倍 | 🟡 M-3 | 同 context 先 L1 后 L3 更省 | 实施时合并 context |
| ARC-9 | `async_validate` 后台校验放大限流 | 🟡 M-4 | 与 M-3 叠加 | 校验主比较改 API vs DOM（非再访问引擎页） |

**实施路径**: Phase -1 抓包 spike → Phase 0 迁独立包(M-1) → Phase 1 只做 L1(M-3) → L2 暂缓(M-2/ARC-1/ARC-2) → 校验改 API vs DOM(H-1)

### 0.5 Screenshot 模块 URL 已修复

| # | 问题 | 状态 | commit |
|---|------|------|--------|
| URL-1 | Hunter 网页 URL `/list?searchValue=` → `/home/list?search=` | ✅ 已修复 | `0e3fcc3` |
| URL-2 | Quake 域名 `quake.360.cn` → `quake.360.net` | ✅ 已修复 | `0e3fcc3` |

### 0.6 问题总览与优先级

```
紧急（阻塞生产）:
  SEC-1  settings.local.json token 泄露评估

高优先（影响稳定性/正确性）:
  CR-3   比较操作符引号确认（ZoomEye/Hunter）
  CR-5   ZoomEye url 映射确认
  ARC-4  API 端点抓包验证

中优先（技术债务/架构改进）:
  SEC-2  API 旧路径 shim 移除（2026-09-01 前）
  TD-1   文件拆分（800 行上限）
  TD-2   函数拆分（50 行上限）
  ARC-6  collection 包迁移
  ARC-7  Extension 域名收窄
  ARC-8  context 合并

低优先（渐进改进）:
  TD-3   错误消息大写
  TD-4   map[string]interface{} 强类型
  CR-1   Quake 排他区间已知限制
  CR-2   Shodan OR 降级已知限制
  CR-4   Shodan header/body 折中已知限制
```

---

## 一、阶段一：核查现有 5 引擎适配器

对照 `SEARCH_ENGINE_SYNTAX_REFERENCE.md` 逐字段核查每个适配器的 `buildCondition` mapping 是否遗漏。本轮语法修正已修复了**翻译逻辑**（分隔符/连接符/引号/布尔），但**字段覆盖度**可能存在遗漏——参考文档列举的字段远多于现有 mapping。

### 1.1 FOFA（`internal/adapter/fofa.go`）

| 现有 mapping | 参考文档字段 | 状态 |
|-------------|------------|------|
| `body`/`title`/`header`/`port`/`protocol`/`ip`/`country`/`region`/`city`/`asn`/`org`/`isp`/`domain`/`host`/`server`/`status_code`/`os`/`app`/`cert`/`url` | — | ✅ 已有 20 条 |
| — | `icon_hash` | ⚠️ 遗漏 |
| — | `fid`（FOFA 指纹 ID） | ⚠️ 遗漏 |
| — | `js_name` / `js_md5` | ⚠️ 遗漏 |
| — | `banner` | ⚠️ 遗漏 |
| — | `after` / `before` | ⚠️ 遗漏 |
| — | `icp` | ⚠️ 遗漏 |
| — | `link` | ⚠️ 遗漏 |
| — | `base_protocol` / `type` | ⚠️ 遗漏 |
| — | `cert.subject` / `cert.issuer` / `cert.subject.org` / `cert.issuer.org` | ⚠️ 遗漏（当前仅有 `cert`） |

**待补充**: 10 个字段映射

### 1.2 Hunter（`internal/adapter/hunter.go`）

| 现有 mapping | 参考文档字段 | 状态 |
|-------------|------------|------|
| `web.body`/`web.title`/`header`/`port`/`protocol`/`ip`/`ip.country`/`ip.province`/`ip.city`/`ip.asn`/`ip.org`/`ip.isp`/`domain`/`web.status_code`/`ip.os`/`app.name`/`header.server`/`cert` | — | ✅ 已有 18 条 |
| — | `web.icon` / `web.favicon` | ⚠️ 遗漏 |
| — | `ip.port`（Hunter 用 `ip.port` 非裸 `port`） | ⚠️ 待确认 |
| — | `ip.domain` | ⚠️ 遗漏 |
| — | `cert.subject` / `cert.issuer` / `cert.subject.cn` / `cert.issuer.cn` | ⚠️ 遗漏 |
| — | `icp.number` / `icp.name` | ⚠️ 遗漏 |
| — | `is.web` / `is_risk` | ⚠️ 遗漏 |
| — | `service` / `banner` | ⚠️ 遗漏 |
| — | `product` / `device.name` / `device.type` | ⚠️ 遗漏 |
| — | `after` / `before` | ⚠️ 遗漏 |

**待补充**: 15 个字段映射

### 1.3 ZoomEye（`internal/adapter/zoomeye.go`）

| 现有 mapping | 参考文档字段 | 状态 |
|-------------|------------|------|
| `http.body`/`title`/`http.header`/`port`/`service`/`ip`/`country`/`subdivisions`/`city`/`asn`/`org`/`isp`/`domain`/`app`/`os`/`device`/`banner`/`http.header.server`/`hostname`/`site`/`http.header.status_code`/`ssl` | — | ✅ 已有 22 条 |
| — | `ver` | ⚠️ 遗漏 |
| — | `ssl.cert.subject.cn` / `ssl.cert.issuer.cn` | ⚠️ 遗漏（当前仅有 `ssl`） |
| — | `webapp` | ⚠️ 遗漏 |
| — | `desc` / `keywords` | ⚠️ 遗漏 |
| — | `iconhash` | ⚠️ 遗漏 |
| — | `subdomain` | ⚠️ 遗漏 |
| — | `time` | ⚠️ 遗漏 |

**待补充**: 8 个字段映射

### 1.4 Quake（`internal/adapter/quake.go`）

| 现有 mapping | 参考文档字段 | 状态 |
|-------------|------------|------|
| `response`/`title`/`headers`/`port`/`service`/`ip`/`country`/`province`/`city`/`asn`/`org`/`isp`/`domain`/`app`/`os`/`server`/`url`/`status_code`/`cert` | — | ✅ 已有 19 条 |
| — | `hostname` | ⚠️ 遗漏 |
| — | `body`（Quake 用 `response` 还是 `body`？） | ⚠️ 待确认 |
| — | `cert.subject.cn` / `cert.issuer.cn` | ⚠️ 遗漏 |
| — | `favicon` | ⚠️ 遗漏 |
| — | `product` / `device` | ⚠️ 遗漏 |
| — | `transport` | ⚠️ 遗漏 |
| — | `notice` / `vuln` / `is_vul` / `is_web` / `is_domain` / `is_ipv6` | ⚠️ 遗漏 |
| — | `time` | ⚠️ 遗漏 |

**待补充**: 12 个字段映射

### 1.5 Shodan（`internal/adapter/shodan.go`）

| 现有 mapping | 参考文档字段 | 状态 |
|-------------|------------|------|
| `http.html`/`http.title`/`port`/`transport`/`ip`/`country`/`region`/`city`/`asn`/`org`/`isp`/`domain`/`hostname`/`product`/`http.status`/`os`/`ssl` | — | ✅ 已有 17 条 |
| — | `http.server` | ⚠️ 遗漏 |
| — | `http.location` | ⚠️ 遗漏 |
| — | `http.favicon.hash` | ⚠️ 遗漏 |
| — | `ssl.cert.subject.cn` / `ssl.cert.issuer.cn` / `ssl.cert.serial` / `ssl.version` / `ssl.ja3.hash` / `ssl.jarm` | ⚠️ 遗漏 |
| — | `ntp.ip` / `ntp.port` | ⚠️ 遗漏 |
| — | `version` | ⚠️ 遗漏 |
| — | `vuln` / `has_screenshot` / `has_vuln` | ⚠️ 遗漏 |

**待补充**: 13 个字段映射

### 1.6 核查汇总

| 引擎 | 现有 | 遗漏 | 完整度 |
|------|------|------|--------|
| FOFA | 20 | 10 | 67% |
| Hunter | 18 | 15 | 55% |
| ZoomEye | 22 | 8 | 73% |
| Quake | 19 | 12 | 61% |
| Shodan | 17 | 13 | 57% |

**行动**: 逐一补充遗漏字段映射 + 对应测试用例。

---

## 二、阶段二：新增搜索引擎

### 2.1 优先级排序

| 优先级 | 引擎 | 理由 | 语法兼容度 | 预估工作量 |
|--------|------|------|-----------|-----------|
| **P1** | **Censys** | 国际主流，API 文档完善，证书搜索强 | 分隔符 `:` + `AND`/`OR`/`NOT`（类 Quake） | 2-3 天 |
| **P1** | **DayDayMap** | 国内平台，语法最丰富，兼容 FOFA/Hunter | 分隔符 `=` + `&&`/`||`（类 FOFA） | 1-2 天 |
| **P2** | **BinaryEdge** | 国际，API 简洁，协议字段丰富 | 分隔符 `:` + 空格/`OR`/`-`（类 Shodan） | 1-2 天 |
| **P2** | **Onyphe** | OQL 语法差异大，但功能独特（暗网/威胁列表） | 分隔符 `:` + `+`(AND) | 2-3 天 |
| **P3** | **GreyNoise** | 威胁情报补充，字段少 API 简单 | 分隔符 `:` + 空格/`OR`/`-` | 1 天 |
| **P3** | **DnsDB** | DNS 历史记录，场景特殊 | 分隔符 `:` + 空格 | 1 天 |

### 2.2 Censys 实施详情

**API**: `search.censys.io/api/v2/hosts/search`
**认证**: API ID + Secret（HTTP Basic Auth）
**分页**: `cursor` 游标分页

**UQL → Censys 字段映射**:

| UQL 字段 | Censys 字段 | 说明 |
|----------|------------|------|
| body | `services.http.response.body` | |
| title | `services.http.response.html_title` | |
| header | `services.http.response.headers.raw` | |
| port | `services.port` | |
| protocol | `services.service_name` | |
| ip | `ip` | |
| country | `location.country_code` | |
| region | `location.province` | |
| city | `location.city` | |
| asn | `autonomous_system.asn` | |
| org | `autonomous_system.name` | |
| isp | — | Censys 无独立 ISP 字段 |
| domain | — | Censys 无独立 domain 字段 |
| host | — | 用 `ip` 代替 |
| server | `services.http.response.headers.Server` | 需精确路径 |
| status_code | `services.http.response.status_code` | |
| os | `operating_system` | |
| app | `services.software.product` | |
| cert | `services.tls.certificates.leaf.subject` | |
| url | — | Censys 无 URL 字段 |

**布尔语法**: `AND`/`OR`/`NOT`（英文词，与 Quake 相同），分隔符 `:`（与 Shodan 相同）
**特殊**: 层级路径用 `.` 分隔（`services.http.response.html_title`），支持正则 `/{pattern}/`

**输出格式**:
```json
{
  "result": {
    "hits": [...],
    "total": 12345,
    "links": { "next": "cursor_token" }
  }
}
```

### 2.3 DayDayMap 实施详情

**API**: 待确认（需注册获取 API Key，官网 `www.daydaymap.com`）
**认证**: API Key
**语法兼容**: 与 FOFA 高度兼容（`=` 分隔、`&&`/`||` 连接）

**UQL → DayDayMap 字段映射**:

| UQL 字段 | DayDayMap 字段 | 兼容语法 |
|----------|---------------|---------|
| body | `web.body` | `body` |
| title | `web.title` | `title` |
| header | `web.header` | `header` |
| port | `ip.port` | `port` |
| protocol | `protocol.service` | `service` |
| ip | `ip` | — |
| country | `ip.country` | `country` |
| region | `ip.province` | `province`/`region` |
| city | `ip.city` | `city` |
| asn | `asn.number` | — |
| org | `org.name` | — |
| isp | `ip.isp` | — |
| domain | `domain` | — |
| server | `web.server` | `server` |
| status_code | `web.status_code` | `status_code`/`code`/`http_status` |
| os | `ip.os` | — |
| app | `app.name` | — |
| cert | `cert.subject.cn` | — |
| url | — | — |

**DayDayMap 特有字段（可选扩展）**:
- `ip.tag`（CDN/蜜罐/云厂商/Starlink）
- `ip.industry`（银行/教育/医疗/工业/金融）
- `brand`/`model`/`manufacturer`（设备品牌/型号/制造商）
- `device.name`/`device.type`/`device.type_sub`
- `cert.is_expired`/`cert.is_trust`/`cert.md5`
- `vul.cve`/`vul.dvb`
- `icp.number`/`icp.name`/`icp.name_prefix`/`icp.webname`

### 2.4 BinaryEdge 实施详情

**API**: `api.binaryedge.io/v2/query/search`
**认证**: API Key（`X-Key` header）
**分页**: `page` 参数

**UQL → BinaryEdge 字段映射**:

| UQL 字段 | BinaryEdge 字段 | 说明 |
|----------|----------------|------|
| body | `body` | |
| title | `title` | |
| header | `header` | |
| port | `port` | |
| ip | `ip` | |
| country | `country` | |
| asn | `asn` | |
| domain | `domain` | |
| os | `os` | |
| app | `product` | |
| cert | `cert` | |
| server | — | 用 `header` 代替 |

**布尔语法**: 空格(AND) / `OR` / `-`(NOT)，与 Shodan 相同

### 2.5 Onyphe 实施详情

**API**: `www.onyphe.io/api/v2/search`
**认证**: API Key（`Authorization: apikey xxx` header）
**分页**: `page` 参数

**UQL → Onyphe 字段映射**:

| UQL 字段 | Onyphe 字段 | 说明 |
|----------|------------|------|
| ip | `ip` | |
| port | `port` | |
| domain | `domain` | |
| hostname | `hostname` | |
| country | `country` | |
| city | `city` | |
| asn | `asn` | |
| org | `organization` | |
| os | `os` | |
| app | `product` | |
| cert | — | |
| banner | — | 通过 `category:datascan` 搜索 |

**布尔语法**: `+`(AND) / 空格(AND) / `OR` / `-`(NOT)
**特殊**: 需加 `category:` 前缀限定搜索类别

### 2.6 GreyNoise 实施详情

**API**: `api.greynoise.io/v3/gnql/search`
**认证**: API Key（`key` 参数或 `X-Api-Key` header）
**分页**: `scroll` 参数

**UQL → GreyNoise 字段映射**:

| UQL 字段 | GreyNoise 字段 | 说明 |
|----------|---------------|------|
| ip | `ip` | |
| country | `metadata.country_code` | |
| org | `metadata.organization` | |
| os | `metadata.os` | |
| app | `metadata.device` | |

**布尔语法**: 空格(AND) / `OR` / `-`(NOT)
**特殊**: GreyNoise 专注威胁情报，字段较少，主要价值在 `classification`/`tag`/`noise`/`riot`/`c2` 等威胁维度

### 2.7 DnsDB 实施详情

**API**: `api.dnsdb.io/lookup/...`（需确认）
**认证**: API Key
**字段**: `domain`/`ip`/`type`/`value`/`time`
**特殊**: 专注 DNS 历史解析记录，与其它引擎功能不重叠

---

## 三、每个引擎实施步骤

```
1. 注册获取 API Key + 阅读官方 API 文档
2. 对照参考文档建立字段映射表
3. 实现 adapter 接口:
   - New{Engine}Adapter(baseURL, apiKey, qps, timeout)
   - Translate(ast *model.UQLAST) (string, error)
   - Search(ctx, query, page, pageSize) (*model.EngineResult, error)
   - Normalize(raw *model.EngineResult) ([]model.UnifiedAsset, error)
   - GetQuota() (*model.QuotaInfo, error)
   - IsWebOnly() bool
4. 实现 WebOnly 适配器（如需要）
5. 编写 table-driven 测试:
   - Translate: 简单条件/NOT/AND/OR/IN/比较操作符/字段映射
   - Normalize: 新格式/旧格式/缺失字段/URL 构建
   - Search: mock HTTP 响应
6. 在 orchestrator.go 注册新引擎
7. go test -race ./internal/adapter/... 通过
8. 更新 docs/SEARCH_ENGINE_SYNTAX.md 基准文档
9. 真机 API 验证（至少 1 条查询命中）
10. 更新 memory 文件
```

---

## 四、字段映射速查

UQL 统一字段 → 各引擎原生字段对照表（含新增引擎）：

| UQL 字段 | FOFA | Hunter | ZoomEye | Quake | Shodan | Censys | DayDayMap | BinaryEdge | Onyphe |
|----------|------|--------|---------|-------|--------|--------|-----------|------------|--------|
| body | `body` | `web.body` | `http.body` | `response` | `http.html` | `services.http.response.body` | `web.body` | `body` | — |
| title | `title` | `web.title` | `title` | `title` | `http.title` | `services.http.response.html_title` | `web.title` | `title` | — |
| header | `header` | `header` | `http.header` | `headers` | `http.html`¹ | `services.http.response.headers.raw` | `web.header` | `header` | — |
| port | `port` | `port` | `port` | `port` | `port` | `services.port` | `ip.port` | `port` | `port` |
| ip | `ip` | `ip` | `ip` | `ip` | `ip` | `ip` | `ip` | `ip` | `ip` |
| country | `country` | `ip.country` | `country` | `country` | `country` | `location.country_code` | `ip.country` | `country` | `country` |
| city | `city` | `ip.city` | `city` | `city` | `city` | `location.city` | `ip.city` | — | `city` |
| org | `org` | `ip.org` | `org` | `org` | `org` | `autonomous_system.name` | `org.name` | — | `organization` |
| asn | `asn` | `ip.asn` | `asn` | `asn` | `asn` | `autonomous_system.asn` | `asn.number` | `asn` | `asn` |
| os | `os` | `ip.os` | `os` | `os` | `os` | `operating_system` | `ip.os` | `os` | `os` |
| app | `app` | `app.name` | `app` | `app` | `product` | `services.software.product` | `app.name` | `product` | `product` |
| cert | `cert` | `cert` | `ssl` | `cert` | `ssl` | `services.tls.certificates.leaf` | `cert.subject.cn` | `cert` | — |
| domain | `domain` | `domain` | `domain` | `domain` | `domain` | — | `domain` | `domain` | `domain` |
| server | `server` | `header.server` | `http.header.server` | `server` | `product`² | — | `web.server` | — | — |
| status_code | `status_code` | `web.status_code` | `http.header.status_code` | `status_code` | `http.status` | `services.http.response.status_code` | `web.status_code` | — | — |

> ¹ Shodan header 无独立 filter，折中映射为 `http.html`
> ² Shodan server 无独立 filter，折中映射为 `product`

---

## 五、阶段三：三层采集架构

> 完整设计见 `docs/THREE_LAYER_COLLECTION_ARCHITECTURE.md`
> 实施计划见 `docs/THREE_LAYER_IMPLEMENTATION_PLAN.md`（5 Phase，15-20 天）

### 当前状态

双路互备（CDP ↔ Extension），两条链路均基于 DOM 解析（L3）。

| 层级 | 状态 | 说明 |
|------|------|------|
| L1 Network (CDP `Network.responseReceived`) | ❌ 缺失 | CDP 仅用 `network.SetCookie`，无网络监听 |
| L2 Hook (Extension fetch/XHR Hook) | ❌ 缺失 | 需 MAIN world 注入 + postMessage 桥（ARC-1/ARC-2） |
| L3 DOM | ✅ 已有 | 5 引擎全覆盖，多级 fallback |

### 实施路径

```
Phase -1: 抓包 spike（验 ARC-4/ARC-5，1-2 天）
    ↓
Phase 0: 迁独立包 internal/collection/（ARC-6，1 天）
    ↓
Phase 1: 只做 L1 Network 层（同 context 复用 ARC-8，3-5 天）
    ↓
Phase 2: L2 Hook 层（暂缓，需解决 ARC-1/ARC-2/ARC-7，5-7 天）
    ↓
Phase 3: 校验层（主比较改 API vs DOM，ARC-9，2-3 天）
    ↓
Phase 4: 浏览器探针 + Network 健康检测（1-2 天）
```

### 关键前置条件

- L1 实施前需 Phase -1 抓包验证各引擎网页端点（ARC-4）
- L2 需解决 MAIN world 注入 + postMessage 桥（ARC-1/ARC-2）
- L2 需收窄 `<all_urls>` 至引擎域名（ARC-7）
- 新增引擎（阶段二）的域名需同步加入 Extension manifest

---

## 六、全局实施时间线建议

```
2026-06 (当前)
  ├── ✅ 5 引擎语法修正 (commit 0e3fcc3)
  ├── 📋 SEC-1: settings.local.json token 评估
  └── 📋 CR-3/CR-5: 比较操作符引号 + ZoomEye url 确认

2026-06 中旬
  ├── 阶段一: 核查补充 5 引擎遗漏字段
  ├── CR-3: 对照官方文档确认比较操作符引号
  └── CR-5: 确认 ZoomEye v2 url 字段

2026-06 下旬
  ├── 阶段二 P1: Censys + DayDayMap 适配器
  ├── ARC-4: Phase -1 抓包 spike
  └── ARC-6: Phase 0 迁 internal/collection/

2026-07 上旬
  ├── 阶段二 P2: BinaryEdge + Onyphe 适配器
  └── 阶段三 Phase 1: L1 Network 层

2026-07 中旬
  ├── 阶段二 P3: GreyNoise + DnsDB 适配器
  ├── TD-1/TD-2: 文件/函数拆分
  └── 阶段三 Phase 2: L2 Hook 层（如需）

2026-07 下旬
  ├── 阶段三 Phase 3: 校验层
  └── ARC-7: Extension 域名收窄

2026-08
  ├── SEC-2: API 旧路径 shim 移除准备
  ├── TD-3/TD-4: 渐进代码质量改进
  └── 阶段三 Phase 4: 浏览器探针

2026-09-01
  └── SEC-2: API 旧路径 shim 移除（sundown deadline）
```

---

## 七、风险与依赖

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| Censys 免费版限制（250 条/月） | 低配额用户无法充分使用 | 配额检查 + 优雅降级提示 |
| DayDayMap API 文档不公开 | 字段映射可能不准确 | 先注册试用，抓包确认 API 格式 |
| Onyphe OQL 语法差异大 | `category:` 前缀需特殊处理 | 在 `buildCondition` 中对 Onyphe 做特殊分支 |
| 新引擎 API 变更频繁 | 字段映射失效 | 每引擎添加版本检查 + 健壮的错误处理 |
| 配置文件引擎枚举 | 新引擎需改配置/注册 | orchestrator 动态注册，配置文件新增 `engines.{name}` 节 |
| 三层采集 L2 MAIN world 限制 | MV3 架构限制 | 两段式注入 + postMessage 桥（ARC-1/ARC-2） |
| API 旧路径 sundown 2026-09-01 | 过期后旧客户端 404 | 提前通知用户迁移，8 月前完成清理 |
| token 泄露风险 | 安全事件 | 评估历史提交，必要时轮换 token |
