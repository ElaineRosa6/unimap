# 插件 / CDP 现状与下一步（2026-08-21）

> 对象：后续 agent。本文只记录当天事实，不把插件活抽当成 CDP 通过，也不把改选择器当成 Bridge 闭环。
> 当前状态（2026-09-04）：本页仍是最近一次人工浏览器核验；2026-08-21 后尚无新的 CollectAndCapture CDP 复跑。当前发布基线见 CURRENT_STATUS_2026-09-04.md。
> 云端试运行仍是 API 查询日更、截图关闭，本轮全部在本机已登录 Chrome 上做。
> 不记录 Cookie 值、ticket、账号。

## 1. 两套路径不要混

| 路径 | 当天怎么验的 | 没验什么 |
|---|---|---|
| **插件** Unimap Screenshot Bridge **0.4.18** | 用户 Chrome（已登录、已 reload）上 `chrome.scripting` 读结果页 DOM | Bridge `tasks/next`、结果页 PNG、SQLite、通知 |
| **CDP** `CollectAndCaptureSearchEngineResult` | 08-21 上午 FOFA/Shodan 走过；Quake/Hunter 07-29；DayDayMap/Censys 08-02 | **今天改完 ExtractJS 之后没有再跑 CDP** |

本机 `127.0.0.1:9222` 在插件核验时段未监听。插件活抽用的是普通 Chrome，不是 CDP profile。

## 2. 插件活页（当天）

查询多为 `port="80"` / `port:80`。登录页、空结果、挑战页未记通过。

| 引擎 | 登录/可搜 | 行选择器 | 条数 | 改选择器后是否再活抽 |
|---|---|---|---:|---|
| FOFA | 已登录 | `.hsxa-meta-data-item` | 10 | 否（之后才拆 `hsxa-host` / `.hsxa-port`） |
| Hunter | 已登录 | `.q-table tbody tr` | 13（含 ICP 脏行） | 否（之后才跳过脏行 / 解析 `tls`） |
| ZoomEye | `.org` 已登录，今日可开 | `.search-result-item-container` | 10（需等 loading） | 否（之后才把 `hostname:port` 写入 host） |
| Quake | 已登录，英文结果页 | `.item-container` | 10 | 否（之后才过滤侧栏聚合、host `--`） |
| Shodan | 能搜 | `.l-search-results .result` | 10 | 布局与 08-21 CDP 校准一致 |
| Censys | Free Plan，能搜 | 当时宽选择器抽出 102 条嵌套 | — | **否**；代码已改为 `a[href*='/hosts/']`，仅单测 |
| DayDayMap | 能搜（`chrome.cookies` 名仍为 0，但结果约 21.6 亿） | `tr.ant-table-row` | **10** | **是：0.4.18 在当前结果页复验**。IP 已去掉「视频监控设备+0」，总数 `2,163,417,935` |

DayDayMap 空 `/searchResult`（无 `keyword`）必须先回首页再填检索，否则会提交「语法不能为空」。

代码：`tools/extension-screenshot` 0.4.18 与 `internal/screenshot/dom_selectors.go` ExtractJS 已对齐。活页 JSON 在 `data/selector-audit-2026-08-21/`（不入库）。

## 3. CDP 定级（改 ExtractJS 之前的最近一次）

| 引擎 | 日期 | 结论 | 证据 |
|---|---|---|---|
| FOFA | 2026-08-21 | 通过：10 条 + 结果页 PNG | [CDP_VERIFICATION_FOFA_SHODAN_2026-08-21.md](CDP_VERIFICATION_FOFA_SHODAN_2026-08-21.md) |
| Shodan | 2026-08-21 | 通过：10 条 + PNG（`.l-search-results` / IPv6 `/host/`） | 同上 |
| Hunter | 2026-07-29 | 通过：10 条 + 截图 | [CDP_VERIFICATION_2026-07-29.md](CDP_VERIFICATION_2026-07-29.md) |
| Quake | 2026-07-29 | 通过：10 条 + 截图 | 同上 |
| DayDayMap | 2026-08-02 | 通过：Bridge 交接后 CDP 10 条 + PNG | [BROWSER_SEVEN_ENGINE_VERIFICATION_2026-08-02.md](BROWSER_SEVEN_ENGINE_VERIFICATION_2026-08-02.md) |
| Censys | 2026-08-02 | 挑战已识别；原生 CDP 到 Cloudflare；`auto` 回退 Bridge 9 条 | 同上 |
| ZoomEye | 2026-08-21 | **受限**：当时 `.org` HTTP 521、`.ai` SSO 超时。`BuildSearchEngineURL` 仍指向 `.org` | FOFA/Shodan 定级文档 |

今天插件在 **zoomeye.org** 打开了结果页并抽出 10 条，与 08-21 CDP 的 521 **不是同一时刻的站点状态**。不能据此改写 08-21 CDP 结论，只能作为「下一步 CDP 可再试 `.org`」的线索。

今天改过的 ExtractJS（FOFA 端口拆分、ZoomEye host:port、Quake `--`、Censys `/hosts/`、DayDayMap `ant-table-row`）**尚未**再用 `CollectAndCaptureSearchEngineResult` 复测。

## 4. 下一步（按顺序，本机，不要动云端截图）

1. **CDP 用当前 ExtractJS 复跑 `CollectAndCaptureSearchEngineResult`**
   优先：DayDayMap、ZoomEye（先 `.org`，521 则记站点受限、不要擅自改 `.ai`）、Censys（挑战则维持 fallback，不要把挑战页当通过）。随后 FOFA / Hunter / Quake（今天改过 JS）。每引擎要非空资产 + 结果页 PNG；登录墙/空结果不得报成功。
2. **插件改后活抽**
   FOFA（host/端口拆分）、Hunter（无 ICP 脏行、`tls`）、Censys（`a[href*='/hosts/']` 非空且去重）、ZoomEye（host 不是 `name:port`）、Quake（无侧栏假行、host `--` 为空）。DayDayMap 0.4.18 已过，不必重开。
3. **不要把本轮写成 Bridge 闭环**
   未跑 `tasks/next`、未截 PNG、未写 SQLite、未发通知。需要时另开，且云端 `screenshot.enabled` 保持 false。
4. **ZoomEye 域名**
   代码搜索 URL 仍是 `.org`。仅当用户明确要求，或 CDP 复测证明 `.org` 不可用且 `.ai` 可登录闭环时，再改 `BuildSearchEngineURL` 和测试。

云端日更范围不变：FOFA / Hunter / DayDayMap 查询 + ICP；Quake/ZoomEye/Shodan/Censys 不进日更。
