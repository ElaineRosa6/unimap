# 全量实施计划 — 空间搜索引擎 + 遗留问题

> **创建日期**: 2026-06-07
> **最后更新**: 2026-06-10
> **状态**: 🔄 阶段一✅ 阶段二✅ (P1/P2/P3全部完成) 阶段三：L1✅ L2暂缓 L3✅
> **基准文档**: `SEARCH_ENGINE_SYNTAX_REFERENCE.md`（12 引擎语法，含归档）、`SEARCH_ENGINE_SYNTAX.md`（UQL→引擎翻译基准）
> **前置依赖**: commit `0e3fcc3`（5 引擎语法修正闭环）
> **来源**: CLAUDE.md 已知待修复事项、code review 发现、memory 遗留缺陷、三层采集架构设计评审、外部语法审计

---

## 目录

- [〇、全局待解决问题清单](#〇全局待解决问题清单)
- [一、阶段一：核查现有 5 引擎适配器](#一阶段一核查现有-5-引擎适配器)
- [二、阶段二：新增搜索引擎](#二阶段二新增搜索引擎)
- [三、每个引擎实施步骤](#三每个引擎实施步骤)
- [四、字段映射速查](#四字段映射速查)
- [五、阶段三：三层采集架构](#五阶段三三层采集架构)
- [六、全局实施时间线建议](#六全局实施时间线建议)
- [七、剩余大项评估与后续实施](#七剩余大项评估与后续实施)
- [八、风险与依赖](#八风险与依赖)

---

## 〇、全局待解决问题清单

> 汇总自 CLAUDE.md 已知待修复事项、adapter code review、memory 遗留缺陷、三层采集架构设计评审。

### 0.1 安全与合规（紧急）

| # | 问题 | 来源 | 状态 | 行动 |
|---|------|------|------|------|
| SEC-1 | admin token 泄露 | memory 2026-06-07 | ✅ 已完成 (commit `3fd93de`) | **定位纠正**：真正泄露点非 settings.local.json（历史 0 次），而是已推送 GitHub 的 `docs/E2E_COLLECTION_VERIFICATION`(commit `3ce543d`) 与本文件(commit `fd583b3`)。已轮换 token + docs 打码 + settings.local.json 加 gitignore/rm --cached。详见 memory `project_sec1_token_rotation_2026-06-08.md` |
| SEC-2 | API 版本化旧路径 shim sundown **2026-09-01** | memory 2026-05-31 | ✅ 已提前完成 (commit `24a37f7`, 2026-06-09) | 移除 `deprecateMiddleware` + 73条 legacy 路由 + middleware 旧路径检查 |

### 0.2 代码质量 — 技术债务（CLAUDE.md 记录）

| # | 问题 | 来源 | 行动 |
|---|------|------|------|
| TD-1 | ~~10 个文件超 800 行~~ ✅ 已全部拆分完成（2026-06-09，最大 `metrics.go` 795 行） | CLAUDE.md Medium | 15 个 commit 拆分 10 个文件，全部 ≤ 795 行 |
| TD-2 | ~~34 个函数超 50 行~~ ✅ 已全部拆分完成（2026-06-09，191→144，最大 `plugin_demo.go:main` 183 行为示例文件） | CLAUDE.md Medium | 47 个函数拆分，39 文件，净减 1159 行 |
| TD-3 | ~~错误消息大写 23 处~~ ✅ 已修复（2026-06-10，2 处非缩写词改为小写，21 处为 HTTP/API/ICP 等缩写词可接受） | CLAUDE.md Low | 逐条审查，非缩写词改为小写 |
| TD-4 | `map[string]interface{}` 强类型（插件接口广泛使用） | CLAUDE.md Low | 渐进重构，定义 `PluginResult` 等结构体 |

### 0.3 适配器 Code Review 遗留（2026-06-07 review）

| # | 问题 | 级别 | 来源 | 行动 |
|---|------|------|------|------|
| CR-1 | Quake `>` 和 `>=` 输出相同 `[N TO *]`（无排他下界） | MEDIUM | code review M-3 | 文档已记录为已知限制，Quake 语法本身不支持排他区间 |
| CR-2 | Shodan OR 降级为 AND（结果集更小） | MEDIUM | code review M-2 | 已加 `logger.Warnf`；长期考虑在 UI 层提示用户"Shodan 不支持跨字段 OR" |
| CR-3 | ZoomEye/Hunter 比较操作符值加引号（`port>"80"` 而非 `port>80`） | MEDIUM | code review M-5/M-6 | ✅ 已确认：Hunter 支持比较运算符且值需引号；ZoomEye 不支持比较运算符，已降级为等值查询（2026-06-09） |
| CR-4 | Shodan `header` 和 `body` 都映射到 `http.html` | LOW | code review L-2 | Shodan 无独立 header filter，已接受折中；文档记录 |
| CR-5 | ZoomEye `url` 映射到 `site`（旧映射未更新） | LOW | code review L-3 | ✅ 已确认：ZoomEye 无 `url` 字段，`site` 是正确映射（2026-06-09） |

### 0.4 采集架构 — 三层采集设计评审遗留（2026-06-06）

> 完整设计见 `docs/THREE_LAYER_COLLECTION_ARCHITECTURE.md`，实施计划见 `docs/THREE_LAYER_IMPLEMENTATION_PLAN.md`

| # | 问题 | 级别 | 要点 | 行动 |
|---|------|------|------|------|
| ARC-1 | L2 hook 在 ISOLATED world，拦不到 fetch/XHR | 🔴 C-1 | MV3 content script 默认 ISOLATED world，不共享 JS 全局 | Phase 1 前置：`world: MAIN` 注入 + postMessage 桥（见 ARC-2） |
| ARC-2 | MAIN world 无 `chrome.runtime` | 🔴 C-2 | 拦截(MAIN) 与 回传(ISOLATED) 互斥 | 两段式注入：MAIN world 拦截 → postMessage → ISOLATED world 回传 |
| ARC-3 | L1 与 L2 同源冗余非正交 | 🟠 H-1 | 都拦同一份 API 响应，改端点/加密同时失效 | 建议先做 L1，L2 暂缓 |
| ARC-4 | API 端点为猜测需抓包验证 | 🟠 H-2 | Shodan 列的是官方 API 非网页端点 | ✅ 已完成：Hunter(`POST /api/search`)、ZoomEye(`GET /api/search`)、Quake(`POST /api/visitor/search/query_string/quake_service`) 为 SPA+XHR；FOFA/Shodan 为 SSR (2026-06-09) |
| ARC-5 | 加密响应体无应对 | 🟠 H-3 | 强反爬引擎 L1/L2 归零退回 DOM | 2026-06-07 更新：5 引擎 Extension 采集已全部打通，H-3 风险评估不变 |
| ARC-6 | collection 代码塞进 `internal/screenshot/` 包 | 🟡 M-1 | 职责膨胀 | ✅ 已完成：`CollectResult`+解析函数迁移到 `internal/collection/` (2026-06-09) |
| ARC-7 | `<all_urls>` 全网注入 | 🟡 M-2 | 与"降低检测"矛盾 | ✅ 已完成：收窄至 10 引擎域名 + localhost (2026-06-09) |
| ARC-8 | L1/L3 独立 context 请求翻倍 | 🟡 M-3 | 同 context 先 L1 后 L3 更省 | ✅ 已完成：`CollectAndCaptureSearchEngineResult` 单次导航 (2026-06-09) |
| ARC-9 | `async_validate` 后台校验放大限流 | 🟡 M-4 | 与 M-3 叠加 | 校验主比较改 API vs DOM（非再访问引擎页） |

**实施路径**: Phase -1 抓包 spike → Phase 0 迁独立包(M-1) → Phase 1 只做 L1(M-3) → L2 暂缓(M-2/ARC-1/ARC-2) → 校验改 API vs DOM(H-1)

### 0.5 Screenshot 模块 URL 已修复

| # | 问题 | 状态 | commit |
|---|------|------|--------|
| URL-1 | Hunter 网页 URL `/list?searchValue=` → `/home/list?search=` | ✅ 已修复 | `0e3fcc3` |
| URL-2 | Quake 域名 `quake.360.cn` → `quake.360.net` | ✅ 已修复 | `0e3fcc3` |

### 0.6 问题总览与优先级

```
✅ 已完成:
  SEC-1  admin token 泄露 → 已轮换+清理 (commit 3fd93de，2026-06-08)

✅ 已完成（2026-06-08 全量核查 + 修复）:
  B-1b   Hunter asn→ip.asn 修正 ✅
  B-1a   FOFA isp 移除 ✅
  B-2    Quake body→response 确认 + header→headers 修正 ✅
  B-3    ZoomEye url→site 确认 ✅
  B-4    比较操作符引号确认（各引擎行为不同）✅
  B-4a   FOFA == 运算符修正 ✅
  B-5    Shodan server→http.server 修正 ✅
  B-6    Shodan header→http.headers_hash 修正 ✅
  B-7    FOFA cert 子字段补充 ✅
  ZoomEye 分隔符确认 (=) ✅

高优先（剩余）:
  ARC-4  API 端点抓包验证 ✅ 已完成 (2026-06-09，5引擎全量：Hunter/ZoomEye/Quake=SPA+XHR，FOFA/Shodan=SSR)

中优先（技术债务/架构改进）:
  SEC-2  API 旧路径 shim 移除 ✅ 已完成 (2026-06-09)
  TD-1   文件拆分（800 行上限）✅ 已完成 (2026-06-09)
  TD-2   函数拆分（50 行上限）✅ 已完成 (2026-06-09，191→144，-1159 行)
  ARC-6  collection 包迁移 ✅ 已完成 (2026-06-09，新建 internal/collection/，迁移 CollectResult+解析函数)
  ARC-7  Extension 域名收窄 ✅ 已完成 (2026-06-09，<all_urls> → 10引擎域名+localhost)
  ARC-8  context 合并 ✅ 已完成 (2026-06-09，CollectAndCapture 单次导航)

低优先（渐进改进，非阻塞）:
  TD-3   错误消息大写 ✅ 已修复 (2026-06-10)
  TD-4   map[string]interface{} 强类型 — 插件接口广泛使用，渐进重构
  CR-1   Quake 排他区间 — 已知限制，Quake 语法不支持排他区间，文档已记录
  CR-2   Shodan OR 降级 — 已知限制，已加 logger.Warnf，长期考虑 UI 提示
  CR-4   Shodan header/body 折中 — 已接受折中，文档已记录
```

---

## 一、阶段一：核查现有 5 引擎适配器

对照 `SEARCH_ENGINE_SYNTAX_REFERENCE.md` 逐字段核查每个适配器的 `buildCondition` mapping 是否遗漏。本轮语法修正已修复了**翻译逻辑**（分隔符/连接符/引号/布尔），但**字段覆盖度**可能存在遗漏——参考文档列举的字段远多于现有 mapping。

### 1.0 ⚠️ 实际代码核查结论（2026-06-08，基于 5 引擎源码 + parser 验证）

> **重要修正**：下方 1.1~1.6 的"遗漏字段"清单系对照参考文档得出，**未经源码与官方语法验证**，机械执行风险高。2026-06-08 核查 5 引擎 `buildCondition`/`mapField` 源码 + `parser.go` 后，结论如下。

**前提事实**：
- UQL parser（`parser.go:379`）**不校验字段名白名单**，任意 `field op value` 均被接受。
- 5 引擎 adapter 字段映射 fallback 均为 `return field`（同名透传）。

**"遗漏字段"按性质分三类**：

| 类别 | 占比 | 性质 | 处置 |
|------|------|------|------|
| **A 同名原生字段** | ~70% (40+) | UQL名==引擎名，parser 不拦 + fallback 透传已能工作；加 mapping 是恒等映射，零行为变化 | **跳过**（冗余） |
| **B 可疑/折中映射** | 5-6 处 | 异名映射，可能是 bug，需官方文档核实 | **核实后修正+加测试**（真正价值，与 CR-3/CR-5 同源） |
| **C 可提升为 UQL 统一维度** | 若干 | 5 引擎名各异的高价值维度（favicon/cert.subject/vuln），提升为 UQL 一等字段才需异名映射 | **产品决策，后续扩展** |

**B 类待官方语法核实清单（开工前必须逐条查证）**：

| # | 引擎 | 现状映射 | 待确认 | 官方文档 |
|---|------|---------|--------|---------|
| B-1 | Hunter | `port → port` | ✅ 已确认 Hunter 短格式 `port="80"` 有效（官方语法），映射正确 | 已查证 |
| B-1a | FOFA | `isp → isp` | ✅ 已修复：从 mapField 移除（改为 passthrough fallback）。测试 `isp field passthrough` 通过 | ✅ 已修复 |
| B-1b | Hunter | `asn → ip.asn` | ✅ 已修复：`ip.asn` → `asn`（commit 待提交）。测试 `field mapping asn short format (B-1b fix)` 通过 | ✅ 已修复 |
| B-2 | Quake | `body → response` | ✅ 已确认映射正确。同时修复 `header → headers`（已删除，改为 passthrough）。测试 `field mapping header passthrough` 通过 | ✅ 已修复 |
| B-3 | ZoomEye | `url → site` | ✅ 已确认 ZoomEye 用 `site` 搜索域名，映射正确 | 已查证 |
| B-4 | ZoomEye/Hunter | `port>"80"`（值带引号） | 部分解答：Hunter `port_count>"2"` 带引号；ZoomEye `port!=80` 不带引号。**FOFA 无数值比较**。各引擎行为不同 | 已查证 |
| B-4a | FOFA | 比较运算符 | ✅ 已修复：`==` 运算符输出从 `field="value"` 修正为 `field=="value"`（精确匹配）。`>` `<` 无 FOFA 等效，保留 fallback 为 `=`。测试 `exact match operator ==` 通过 | ✅ 已修复 |
| B-5 | Shodan | `server → product` | ✅ 已修复：`product` → `http.server`。测试 `field mapping server to http.server (B-5 fix)` 通过 | ✅ 已修复 |
| B-6 | Shodan | `header → http.html` | ✅ 已修复：`http.html` → `http.headers_hash`。测试 `field mapping header to http.headers_hash (B-6 fix)` 通过 | ✅ 已修复 |
| B-7 | FOFA | `cert.subject.cn` / `cert.issuer.cn` | ✅ 已修复：补充 cert.subject.cn 和 cert.issuer.cn 映射。测试 `field mapping cert.subject.cn` 和 `field mapping cert.issuer.cn` 通过 | ✅ 已修复 |

**C 类候选（favicon 跨引擎统一维度示例）**：
FOFA `icon_hash` / ZoomEye `iconhash` / Shodan `http.favicon.hash` / Quake `favicon` / Hunter `web.icon`

**当前状态**：✅ B 类映射修正已闭环；C 类候选保留为后续产品扩展。

---

### 1.1 FOFA（`internal/adapter/fofa.go`）

> **2026-06-08 更新**：基于 FOFA 官方语法（用户查证）重新标注。

| 现有 mapping | 官方语法确认 | 分类 | 行动 |
|-------------|------------|------|------|
| `body`/`title`/`header`/`port`/`protocol`/`ip`/`country`/`region`/`city`/`asn`/`org`/`domain`/`host`/`server`/`status_code`/`os`/`app`/`cert` | ✅ 官方字段 | 已有 18 条 | 无需改动 |
| `url → host` | FOFA 无 `url` 字段，`host` 是合理折中 | ✅ 已有 | 无需改动 |
| `isp → isp` | ⚠️ FOFA 官方语法**无此字段** | B-1a | ✅ 已移除 |
| — | `icon_hash` / `fid` / `js_name` / `js_md5` / `icp` / `banner` / `after` / `before` / `base_protocol` / `type` | A 类同名 | parser fallback 透传已能工作，无需硬编码映射 |
| — | `cert.subject.cn` / `cert.issuer.cn` / `cert.domain` / `cert.subject.org` / `cert.issuer.org` | B-7 子字段 | ✅ 已补充 cert 子字段映射 |
| — | `link` | ❌ 不在官方语法 | 旧文档遗留，已从参考手册删除 |

**FOFA 状态**: ✅ B-1a（isp 已移除）+ B-7（cert 子字段已补充）+ B-4a（`==` 运算符已修正）

### 1.2 Hunter（`internal/adapter/hunter.go`）

> **2026-06-08 更新**：基于 Hunter 官方语法（用户查证）重新标注。

| 现有 mapping | 官方语法确认 | 分类 | 行动 |
|-------------|------------|------|------|
| `web.body`/`web.title`/`header`/`port`/`protocol`/`ip`/`ip.country`/`ip.province`/`ip.city`/`ip.org`/`ip.isp`/`domain`/`web.status_code`/`ip.os`/`app.name`/`header.server`/`cert` | ✅ 官方字段（短格式均有效） | 已有 17 条 | 无需改动 |
| `url → host` | Hunter 无 `url`，`host` 未列于官方语法（`domain` 才是正确字段） | ⚠️ 待确认 | host 透传是否被 Hunter 接受 |
| `asn → ip.asn` | ⚠️ Hunter 官方语法无 `ip.asn`，应用 `asn`（短格式）或 `as.number`（全格式） | B-1b | ✅ 已修正为 `asn → asn` |
| — | `web.icon` / `web.similar` / `web.tag` | A 类同名 / 付费 | parser fallback 已能工作（`web.icon` 需付费） |
| — | `icp.number` / `icp.name` / `icp.web_name` / `icp.type` | A 类同名 | parser fallback 已能工作 |
| — | `is.web` / `is_domain` | A 类同名 | parser fallback 已能工作 |
| — | `protocol.banner` | Hunter banner 字段名（非裸 `banner`） | ⚠️ 透传 `banner` 可能无效，需确认 Hunter 是否接受裸 `banner` |
| — | `cert.subject.suffix` / `cert.subject_org` / `cert.issuer_org` / `cert.serial_number` / `cert.is_expired` / `cert.is_trust` | A 类 / 付费 | cert 子字段（`cert.subject`/`cert.issuer` 已有，子字段同名透传） |
| — | `vul.cve` / `vul.gev` / `vul.state` | A 类同名 | parser fallback 已能工作 |
| — | `after` / `before` | A 类同名 | parser fallback 已能工作 |
| — | `as.name` / `as.org` | A 类同名 | parser fallback 已能工作 |

**Hunter 状态**: ✅ B-1b 已修正；`banner`/`host` 透传作为兼容行为保留，后续按真机/API 行为观察。

### 1.3 ZoomEye（`internal/adapter/zoomeye.go`）

> **2026-06-08 更新**：基于 ZoomEye 官方语法（用户查证）重新标注。
> **重要**：ZoomEye 分隔符为 `=`（非 `:`），值需引号包裹。

| 现有 mapping | 官方语法确认 | 分类 | 行动 |
|-------------|------------|------|------|
| `http.body`/`title`/`http.header`/`port`/`service`/`ip`/`country`/`subdivisions`/`city`/`asn`/`isp`/`app`/`os`/`device`/`http.header.server`/`hostname`/`site`/`http.header.status_code`/`ssl` | ✅ 官方字段 | 已有 19 条 | 无需改动 |
| `url → site` | ✅ 已确认 ZoomEye 用 `site` 搜索域名（B-3 已确认） | B-3 已确认 | 映射正确 |
| `domain → domain` | ✅ ZoomEye 官方有 `domain` 字段 | ✅ | 无需改动 |
| `org → org` | ✅ ZoomEye 官方 `org` 有效（同 `organization`） | ✅ | 无需改动 |
| `cert → ssl` | ✅ ZoomEye 用 `ssl` 搜索证书 | ✅ | 映射正确 |
| `banner → banner` | ⚠️ `banner` 在 ZoomEye 官方语法中**未列为主要字段**（仅用于非 HTTP 协议报文） | ⚠️ | 保留，可能仅对非 HTTP 协议有效 |
| `protocol → service` | ✅ ZoomEye 用 `service` 搜索协议名 | ✅ | 映射正确 |
| — | `ver` / `webapp` / `desc` / `keywords` / `iconhash` / `subdomain` / `time` | A 类同名 | parser fallback 已能工作 |
| — | `industry` / `is_honeypot` / `icp.number` / `icp.name` | A 类同名 | parser fallback 已能工作 |
| — | `ssl.cert.subject.cn` / `ssl.cert.issuer.cn` / `ssl.cert.serial` / `ssl.version` / `ssl.jarm` / `ssl.ja3s` | A 类同名 | parser fallback 已能工作 |
| — | `http.body_hash` / `http.header_hash` / `http.header.version` | A 类同名 | parser fallback 已能工作 |
| — | `dig` / `filehash` / `after` / `before` / `cidr` / `product` / `protocol` | A 类同名 | parser fallback 已能工作 |

**ZoomEye 状态**: ✅ 已确认 adapter 的 `buildCondition` 输出 `field="value"` 格式（`=` 分隔符），与官方语法一致

### 1.4 Quake（`internal/adapter/quake.go`）

> **2026-06-08 更新**：基于 Quake 官方语法（用户查证）重新标注。

| 现有 mapping | 官方语法确认 | 分类 | 行动 |
|-------------|------------|------|------|
| `title`/`port`/`service`/`ip`/`country`/`province`/`city`/`asn`/`org`/`isp`/`domain`/`app`/`os`/`server`/`cert` | ✅ 官方字段 | 已有 15 条 | 无需改动 |
| `body → response` | ✅ 已确认 Quake 正文字段为 `response`（非 `body`）| B-2 已确认 | 映射正确，无需改动 |
| `header → headers` | ⚠️ Quake 官方语法**无 `header`/`headers` 字段** | B-2 follow-up | ✅ 已移除，`header` 走透传兼容 |
| `url → url` | Quake 官方语法**无 `url` 字段** | ⚠️ | 透传给 API 可能被忽略 |
| `status_code → status_code` | Quake 官方语法**无 `status_code` 字段** | ⚠️ | 透传给 API 可能被忽略 |
| `host → domain` | ✅ `host` 透传无意义（Quake 用 `domain`），但当前已映射为 `domain` | ✅ | 无需改动 |
| — | `hostname` / `transport` / `is_ipv6` / `is_domain` | A 类同名 | parser fallback 已能工作 |
| — | `country_cn` / `province_cn` / `city_cn` | A 类同名（Quake 特有中文字段） | parser fallback 已能工作 |
| — | `owner` / `icp_nature` | A 类同名 | parser fallback 已能工作 |
| — | `cert.subject.cn` / `cert.issuer.cn` | A 类同名 | parser fallback 已能工作 |

**Quake 状态**: ✅ `header → headers` 已移除；`url`/`status_code` 透传行为保留为非阻塞观察项。

### 1.5 Shodan（`internal/adapter/shodan.go`）

> **2026-06-08 更新**：基于 Shodan 官方语法（用户查证）重新标注。

| 现有 mapping | 官方语法确认 | 分类 | 行动 |
|-------------|------------|------|------|
| `http.html`/`http.title`/`port`/`transport`/`ip`/`country`/`region`/`city`/`asn`/`org`/`domain`/`hostname`/`product`/`http.status`/`os`/`ssl` | ✅ 官方字段 | 已有 16 条 | 无需改动 |
| `server → product` | 🔴 **错误**：Shodan 有独立 `http.server` 字段搜索 Server 头，`product` 搜索的是产品/软件名（不同概念） | B-5 已确认 | ✅ 已修正为 `http.server` |
| `header → http.html` | 🔴 **错误**：`http.html` 是正文内容搜索（非 header）。Shodan **无独立 header 内容搜索**（仅有 `http.headers_hash` 哈希） | B-6 已确认 | ✅ 已修正为 `http.headers_hash` |
| `isp → isp` | ⚠️ `isp` 不在 Shodan 官方过滤器列表中 | ⚠️ | 保留兼容（可能隐式支持），文档不收录 |
| `app → product` | ✅ Shodan 用 `product` 搜索软件名 | ✅ | 映射正确 |
| — | `http.server` / `http.location` / `http.favicon.hash` | B-5/B-6 相关 | ✅ `http.server` 已补充，其余同名透传 |
| — | `ssl.cert.subject.cn` / `ssl.cert.issuer.cn` / `ssl.cert.serial` / `ssl.cert.fingerprint` / `ssl.version` / `ssl.ja3s` / `ssl.jarm` | A 类同名 | parser fallback 已能工作 |
| — | `vuln` / `has_screenshot` / `has_vuln` / `has_ssl` / `has_ipv6` | A 类同名 | parser fallback 已能工作 |
| — | `http.component` / `http.waf` / `cloud.provider` / `cloud.region` / `screenshot.label` / `cpe` / `link` | A 类同名 | parser fallback 已能工作 |

**Shodan 状态**: ✅ B-5（`server → http.server`）+ B-6（`header → http.headers_hash`）已修正；跨字段 OR 降级为 AND 是已接受限制。

### 1.6 核查汇总（2026-06-08 基于官方语法更新）

| 引擎 | 现有映射 | 确认正确 | 确认 bug | 待处理 |
|------|---------|---------|---------|--------|
| FOFA | 20 条 | 18 条 ✅ | `isp` 不存在(B-1a) | B-7 cert 子字段 + B-4a 比较运算符 |
| Hunter | 18 条 | 17 条 ✅ | `asn→ip.asn`(B-1b) | banner/host 透传确认 |
| ZoomEye | 22 条 | 21 条 ✅ | — | 分隔符 `=` vs `:` 确认 |
| Quake | 19 条 | 16 条 ✅ | `header→headers` | url/status_code 透传确认 |
| Shodan | 17 条 | 16 条 ✅ | `server→product`(B-5)、`header→http.html`(B-6) | isp 兼容确认 |

**A 类同名字段**：5 引擎合计 60+ 字段通过 parser fallback 透传已能工作，无需硬编码映射。

**行动**：修复 4 个确认 bug（B-1b/B-5/B-6/header→headers）+ 对应测试用例。

---

## 二、阶段二：新增搜索引擎

### 2.1 优先级排序

| 优先级 | 引擎 | 理由 | 语法兼容度 | 预估工作量 |
|--------|------|------|-----------|-----------|
| **P1** | **Censys** | 国际主流，API 文档完善，证书搜索强 | 分隔符 `:` + `AND`/`OR`/`NOT`（类 Quake） | ✅ 已完成 |
| **P1** | **DayDayMap** | 国内平台，语法最丰富，兼容 FOFA/Hunter | 分隔符 `=` + `&&`/`||`（类 FOFA） | ✅ 已完成 |
| ~~P2~~ | ~~**BinaryEdge**~~ | ⚠️ **已关闭**（2025-03-31），API 不可用 | — | 代码保留，默认禁用 |
| **P2** | **Onyphe** | OQL 语法差异大，但功能独特（暗网/威胁列表） | 分隔符 `:` + `+`(AND) | ✅ 已完成 |
| **P3** | **GreyNoise** | 威胁情报补充，字段少 API 简单 | 分隔符 `:` + 空格/`OR`/`-` | ✅ 已完成 |
| ~~P3~~ | ~~**DnsDB**~~ | ⚠️ **已停用**，服务不可用 | — | 不实施 |

### 2.2 Censys 实施详情

> **状态**: ✅ 已完成（2026-06-08）
> **文件**: `internal/adapter/censys.go`（462 行）+ `censys_test.go`（813 行，36 测试）
> **配置**: `config.yaml.example` 新增 censys 节 + config.go + 3 个入口注册
> **测试**: `go test -race ./internal/adapter/` 全绿

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

> **状态**: ✅ 已完成（2026-06-08）
> **文件**: `internal/adapter/daydaymap.go` + `daydaymap_test.go`（37 测试）

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

> **状态**: ✅ 已完成（2026-06-08）
> **文件**: `internal/adapter/onyphe.go` + `onyphe_test.go`（47 测试）
> **注意**: 字段映射已修正——移除 Onyphe 不支持的 country/city/os 字段

**API**: `www.onyphe.io/api/v2/simple/search`
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

> **状态**: ✅ 已完成（2026-06-09）
> **文件**: `internal/adapter/greynoise.go` + `greynoise_test.go`
> **配置**: `config.yaml.example` 新增 greynoise 节 + config.go + 3 个入口注册

**API**: `api.greynoise.io/v3/experimental/gnql`
**认证**: API Key（`key` header）
**分页**: 不支持偏移分页，使用 `scroll` 参数或一次性返回

**UQL → GreyNoise 字段映射**:

| UQL 字段 | GreyNoise 字段 | 说明 |
|----------|---------------|------|
| ip | `ip` | |
| classification / class | `classification` | malicious/suspicious/benign/unknown |
| tag / tags | `tags` | 活动标签 |
| org / isp | `metadata.organization` | |
| os | `metadata.os` | |
| country | `metadata.country` | |
| city | `metadata.city` | |
| category | `metadata.category` | |
| port | `raw_data.scan.port` | |
| protocol | `raw_data.scan.protocol` | |
| noise | `noise` | 是否为互联网噪音 |
| riot | `riot` | 是否为常见业务服务 |
| spoofable | `spoofable` | IP 是否可被欺骗 |
| vpn / vpn_service | `vpn_service` | VPN 服务名 |
| first_seen | `first_seen` | |
| last_seen | `last_seen` | |
| asn | `metadata.asn` | |

**布尔语法**: 空格(AND) / `OR` / `-`(NOT)
**特殊**: GreyNoise 专注威胁情报，字段较少，主要价值在 `classification`/`tags`/`noise`/`riot` 等威胁维度。比较操作符（> >= < <=）不支持，降级为等值查询。

### 2.7 DnsDB 实施详情 ⚠️ 已停用

> **状态**: ❌ 不实施（DnsDB 服务已停止）

**API**: `api.dnsdb.io/lookup/...`（已不可用）
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

L1 Network 层已实现（ZoomEye/Hunter/Quake），FOFA/Shodan 保持 L3 DOM。combined collect+capture 路径已集成 L1。

| 层级 | 状态 | 说明 |
|------|------|------|
| L1 Network (CDP `Network.responseReceived`) | ✅ 已实现 | ZoomEye/Hunter/Quake 三引擎支持，2s bounded wait + DOM fallback |
| L2 Hook (Extension fetch/XHR Hook) | ⏸ 暂缓 | 需 MAIN world 注入 + postMessage 桥（ARC-1/ARC-2），非阻塞 |
| L3 DOM | ✅ 已有 | 5 引擎全覆盖，多级 fallback，L1 失败时自动降级 |

### 实施路径

```
Phase -1: 抓包 spike ✅ (ARC-4，2026-06-09)
    ↓
Phase 0: 迁独立包 internal/collection/ ✅ (ARC-6，2026-06-09)
    ↓
Phase 1: L1 Network 层 ✅ (3 引擎 CDP 拦截 + combined collect+capture 集成)
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
  ├── 阶段二 P3: GreyNoise 适配器 ✅ 已完成（DnsDB 已停用，不实施）
  ├── TD-1/TD-2: 文件/函数拆分
  └── 阶段三 Phase 2: L2 Hook 层（如需）

2026-07 下旬
  ├── 阶段三 Phase 3: 校验层
  └── ARC-7: Extension 域名收窄

2026-06-09
  ├── SEC-2: API 旧路径 shim 移除 ✅ 已提前完成 (commit 24a37f7)
  ├── TD-1: 10个大文件拆分 ✅ 已完成 (全部 ≤ 795 行)
  └── TD-2: 47个函数拆分 ✅ 已完成 (191→144，净减 1159 行)

2026-08
  ├── TD-3/TD-4: 渐进代码质量改进（剩余4文件拆分）
  └── 阶段三 Phase 4: 浏览器探针
```

---

## 七、剩余大项评估与后续实施

> 更新日期：2026-06-10。当前剩余项都不是小修补：要么影响跨包数据契约，要么依赖真实浏览器/真实账号验证。原则是先保留现有稳定路径，再用可回滚的阶段性改造降低风险。

### 7.1 剩余项总览

| 项目 | 当前状态 | 难度 | 影响面 | 外部依赖 | 建议 |
|------|----------|------|--------|----------|------|
| TD-4/L-05 `map[string]interface{}` 强类型 | 插件接口、任务 payload、扩展桥接、部分 API 响应仍使用动态 map | 中-高 | 插件兼容性、JSON 契约、测试 fixture、前后端交互 | 无外部服务依赖 | 渐进迁移，先边界后内部；保留 `Extra`/`Raw` 扩展字段 |
| L2 Hook（Extension fetch/XHR Hook） | 设计明确但暂缓 | 高 | Extension MV3、消息桥、采集解析、任务安全边界 | 真实网页端点、SPA 行为、反爬状态 | 只有当 L1/L3 证据显示收益明确时实施 |
| Quake/Shodan Extension stealth | 方案已成型，需真机验证 | 高 | Extension、登录态、速率控制、blocked/captcha 状态、用户提示 | 真实账号、真实浏览器、目标站反爬策略 | 若真实采集是业务关键，优先做阶段 1-3 |

已接受限制不再作为代码任务推进：CR-1 Quake 无排他下界语法；CR-2 Shodan 跨字段 OR 降级为 AND（已 `Warnf`）；CR-4 Shodan header/body 映射折中。

### 7.2 TD-4/L-05 强类型改造评估

不建议全局一次性替换 `map[string]interface{}`。当前动态 map 分布在插件接口、调度任务参数、Extension bridge payload、Web API DTO、测试 fixture 等位置，其中一部分是对外契约，直接替换容易造成插件和前端兼容性回归。

推荐实施顺序：

1. **盘点分类**：按“公开 JSON 契约 / 插件接口 / 内部临时 map / 测试数据”分类，而不是按包名机械替换。
2. **先定义边界结构体**：优先补 `PluginConfig`、`PluginResult`、`HookPayload`、`TaskPayload`、`BridgeTask`、`BridgeResult` 等边界类型。
3. **保留扩展字段**：结构体中保留 `Extra map[string]any` 或 `Raw json.RawMessage`，避免把插件生态和引擎差异封死。
4. **兼容适配层**：旧 map 输入先进入 adapter 转结构体；对外 JSON 字段名不变，避免 UI/API 同步大改。
5. **按模块迁移**：插件接口 → scheduler payload → screenshot/extension bridge → Web API DTO → 内部辅助函数。
6. **删除死 map**：等测试和调用方全部改完，再移除兼容 helper。

建议验收标准：

- 公共 API JSON 字段名不破坏，旧插件仍可加载或给出明确兼容错误。
- 新增结构体覆盖核心业务字段，开放字段只用于真正不稳定的 metadata。
- `go test ./...` 通过；涉及桥接协议时补充至少 1 个序列化兼容测试。
- 文档同步更新插件开发指南或相关 API 文档，标注兼容期。

### 7.3 L2 Hook 实施评估

L2 Hook 的目标是从页面自身的 `fetch`/`XMLHttpRequest` 中捕获响应，但 MV3 content script 默认在 ISOLATED world，不能直接改写页面上下文中的 `window.fetch`。正确架构必须是 MAIN world hook + ISOLATED world bridge：MAIN world 负责拦截，`window.postMessage` 传出；ISOLATED world 校验消息并通过 `chrome.runtime` 回传扩展后台。

当前 L1 Network 已覆盖 ZoomEye/Hunter/Quake，FOFA/Shodan 走 L3 DOM，已有可用采集路径。因此 L2 不应作为默认下一步，只有满足以下任一条件才值得启动：

- L1 端点频繁变化或响应被 CDP 侧不可读，但页面 JS 仍可拿到明文数据。
- L3 DOM 因虚拟列表/懒加载导致稳定性不足，且 L2 能显著提升字段完整度。
- 某个新引擎必须依赖页面内部解密后的响应，L1 无法解析。

推荐实施阶段：

1. **单引擎 spike**：只选一个 SPA 引擎（优先 ZoomEye/Hunter，避免先碰 Quake 反爬变量），验证 MAIN world hook 能捕获目标响应。
2. **消息协议**：定义 `{source, taskId, engine, url, status, headers, bodyHash, body}`，加入 schema 校验、大小限制和 task correlation。
3. **安全边界**：只允许引擎域名注入；postMessage 必须校验 `origin`、`source`、任务 id，避免页面伪造采集结果。
4. **解析与去重**：L2 结果进入 `internal/collection/`，与 L1/L3 做统一归并，不复制一套解析逻辑。
5. **回退策略**：L2 timeout 或协议异常时自动回退 L1/L3，并记录 telemetry，不阻塞用户查询。

建议验收标准：

- 单引擎真实页面可稳定捕获目标响应，且不影响页面正常搜索。
- 注入域名白名单明确，无 `<all_urls>` 泛化回退。
- 有 bounded buffer/response size 上限，避免大响应撑爆扩展或后端。
- L2 失败时结果状态可观测，用户仍能拿到 L1/L3 fallback。

### 7.4 Quake/Shodan stealth 实施评估

Quake/Shodan 的核心问题不是语法翻译，而是真实浏览器链路下的反爬、登录态和行为限制。该项必须以 Extension 真实浏览器路径为主，CDP stealth 只能作为低优先级降级方案；原因是 CDP/runtime/TLS/headless 指纹的可见面更大，投入产出较差。

推荐实施阶段：

1. **基础 stealth 注入**：在引擎域名 `document_start` 注入 webdriver/plugins/languages/permissions 等最小脚本，先验证 Quake 拦截页是否减少。
2. **blocked/login/captcha 状态机**：统一返回 `BLOCKED_BY_ANTI_BOT`、`LOGIN_REQUIRED`、`CAPTCHA_REQUIRED`，并带截图和页面特征，避免把反爬误报成空结果。
3. **智能等待**：`waitForResults` 等待“结果行 / 空状态 / blocked / login / captcha”之一，不再依赖固定 sleep。
4. **限流与冷却**：Quake/Shodan 独立令牌桶，Quake 初始建议 `qps=0.5`、`cooldown=120s`；连续 blocked 后进入更长冷却。
5. **人工介入**：验证码只提示和等待用户手动处理，不实现自动打码或激进绕过。

建议验收标准：

- 已登录真实 Chrome + 已配对 Extension 下，Quake 查询页面非拦截页，能提取至少 1 条真实资产。
- 遇到登录失效、验证码、反爬拦截时，后端和 UI 能展示明确状态，不写入误导性空结果。
- 限流配置可调整，任务取消/超时能释放等待。
- Shodan OR 降级、header/body 折中等已知限制仍按当前文档处理，不混入 stealth 改造。

### 7.5 推荐优先级

1. **Quake/Shodan Extension stealth 阶段 1-3**：如果真实采集能力是近期目标，这是唯一直接改善成功率的路径。
2. **TD-4/L-05 强类型渐进改造**：作为代码质量工作穿插推进，优先做边界类型和兼容测试，不打断功能交付。
3. **L2 Hook**：只有在 L1/L3 telemetry 证明收益后启动；否则保持设计冻结，避免扩大 Extension 复杂度。

## 八、风险与依赖

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| Censys 免费版限制（250 条/月） | 低配额用户无法充分使用 | 配额检查 + 优雅降级提示 |
| DayDayMap API 文档不公开 | 字段映射可能不准确 | 先注册试用，抓包确认 API 格式 |
| Onyphe OQL 语法差异大 | `category:` 前缀需特殊处理 | 在 `buildCondition` 中对 Onyphe 做特殊分支 |
| 新引擎 API 变更频繁 | 字段映射失效 | 每引擎添加版本检查 + 健壮的错误处理 |
| 配置文件引擎枚举 | 新引擎需改配置/注册 | orchestrator 动态注册，配置文件新增 `engines.{name}` 节 |
| 三层采集 L2 MAIN world 限制 | MV3 架构限制 | 两段式注入 + postMessage 桥（ARC-1/ARC-2） |
| ~~API 旧路径 sundown 2026-09-01~~ | ✅ 已提前完成 (2026-06-09) | shim 已移除，所有消费者已迁移到 /api/v1/ |
| token 泄露风险 | 安全事件 | 评估历史提交，必要时轮换 token |
