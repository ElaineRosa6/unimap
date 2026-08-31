# FOFA / Shodan / ZoomEye 原生 CDP 定级（2026-08-21）

> 关联待办：RW-03 / 计划书 C1。Chrome 153，CDP `127.0.0.1:9222`，专用 profile `data/chrome-cdp-grade-human`。
> 路径：`Manager.CollectAndCaptureSearchEngineResult`（L3 DOM + 同页 PNG）。不记录 Cookie 值、ticket、账号。
> ZoomEye 按用户结论视为站点问题，本轮不强制登录闭环。

查询：FOFA `port="80" && country="CN"`；Shodan `port:80 country:"CN"`。

## 定级结论

| 引擎 | 登录态 | 打开结果页 | 非空结构化资产 | 结果页 PNG | 定级 |
|---|---|---|---|---|---|
| FOFA | 通过（头像，非登录页） | 通过 | 通过：10 条，页面合计 93,367,910 | 通过 | **通过** |
| Shodan | 通过（Account，非 Login） | 通过，TOTAL RESULTS 约 517 万 | 通过：10 条（校准后，含 IPv6 `/host/`） | 通过 | **通过** |
| ZoomEye | 未完成 | `.org` HTTP 521；`.ai` 登录回调 `/login?ticket=` 后 `/api/promise` ~61s，页面 Query timeout | 未执行采集 | — | **受限**（站点/SSO，非选择器） |

登录页、验证码页、空结果均未记为通过。

## FOFA

| 步骤 | 结果 |
|---|---|
| Cookie | 15 个（含 `fofa_token` / `user` / nosec CAS，值不记录） |
| 搜索 URL | `https://fofa.info/result?qbase64=...` |
| LoginWall / challenge | false / false |
| DOM | 10 条，method=dom，rows=10 |
| 首条 | host `webmail03.yzu.edu.tw`，port 80 |
| 截图 | 检索结果列表、查询框为原语句、非登录页 |

证据：`data/cdp-verify/app-screenshots/2026-08-21-verify_fofa_20260820/search-engine-results/`

## Shodan

首轮（选择器修复前）：结果页可见，ExtractJS 0 条。根因是（1）页面为 `.l-search-results .result`，没有 `.row` 包裹；（2）Go raw string 里的 `/host/` 正则在 JS 中非法；（3）IPv6 链接为 `/host/2600:…` 与 `http://[v6]:80`；（4）总数在 `h4.total-results`。

修复后复测：

| 步骤 | 结果 |
|---|---|
| Cookie | 3 个（`session` 等，值不记录） |
| 搜索 URL | `https://www.shodan.io/search?query=...` |
| LoginWall / challenge | false / false |
| 标题 | `port:80 country:"CN" - Shodan Search` |
| DOM | 10 条，method=dom，rows=10，total=5,172,442 |
| 首条 | IPv6 `2600:9000:204d:200:16:a16f:9e40:93a1` port 80 |

证据：`data/cdp-verify/app-screenshots/2026-08-21-verify_shodan_20260820/search-engine-results/`

## ZoomEye

- `https://www.zoomeye.org/` 本机 HTTP 521（源站不可达）。
- 现官网 `https://www.zoomeye.ai/` 可开首页。点 Login 后回到 `/login?ticket=`，`/api/promise` 约 61 秒，UI「Query timeout」。用户判定为站点问题。
- UniMap 代码搜索 URL 仍指向 `.org`，即使登录 `.ai` 也不能走现有 `BuildSearchEngineURL` 闭环。
- 不把空白回调页或 521 记成采集成功。

## 未做

- 未写入 `configs/config.yaml`。
- 未跑 SQLite 合并查询工作流、未发图片通知。
- 未改 ZoomEye `.org` → `.ai`（需单独改代码和测试，本轮只定级）。
