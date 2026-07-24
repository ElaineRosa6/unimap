# UniMap 当前剩余工作清单

> 最后核对：2026-07-24。本文是当前本地待办入口；实施顺序和按旬排期见
> [后续实施计划](IMPLEMENTATION_PLAN_2026-07-23.md)。历史审计、计划和 E2E 报告只表达对应日期的事实。
> 完成项必须同时满足代码、测试和所声明后端的真实验收，不能因存在选择器、接口或单元测试就标记完成。

## 1. 引擎能力事实矩阵

| 引擎 | 稳定 Web UI | API 适配 | Bridge 结构化采集实测 | CDP 结构化采集实测 |
|---|---|---|---|---|
| FOFA | 已接入 | 已有适配器；历史另有真实 API 证据 | 2026-07-15 通过 | 未执行 |
| Hunter | 已接入 | 已有适配器 | 2026-07-15 通过 | 未执行 |
| ZoomEye | 已接入 | 已有适配器 | 2026-07-15 通过 | 未执行 |
| Quake | 已接入 | 已有适配器 | 2026-07-15 通过 | 未执行 |
| Shodan | 已接入 | 已有适配器 | 2026-07-15 因登录态失效未通过 | 未执行 |
| Censys | 未接入 | 已完成，2026-06-27 API 实机验证通过 | 未完成、未测试 | 未完成、未测试 |
| DayDayMap | 未接入 | 已完成，2026-06-27 API 实机验证通过 | 未完成、未测试 | 未完成、未测试 |

说明：

- Censys、DayDayMap 的扩展/Go DOM 泛化选择器只是未接通占位；
- Bridge 和 CDP 搜索 URL 构造目前都只接受五个稳定引擎；
- Censys、DayDayMap 的 Go DOM 条目没有结构化 `ExtractJS`；
- `live_bridge_e2e` 白名单只有五个稳定引擎；
- 2026-07-23 已修复 Censys、DayDayMap 的能力误报：缺少完整 API 凭据时不再注册 Web-only adapter；两者仍未实现 Bridge/CDP；
- 普通网页 CDP 截图通过不等于测绘引擎 CDP 结构化采集通过。

## 2. P1：下一阶段必须完成

### RW-01 生产配置持久化与秘密录入

状态：**已完成（2026-07-23）**。

生产 Compose 已改用 `unimap_config:/app/runtime-config` 可写命名卷。入口脚本仅在
`config.yaml` 不存在时从镜像内 `config.prod.yaml` 初始化，权限设为 `0600`；应用通过
`UNIMAP_CONFIG_PATH` 读取运行配置，支持同目录临时文件和原子替换。

完成标准：

- 使用受保护的可写运行配置目录或等价秘密存储；**已完成**
- 不把秘密写入 Git、日志或响应；
- 不可写模式下页面明确提示并禁用保存；**仍待增强**
- 保存、重启、回滚和备份恢复测试通过；**已完成**

阿里云实机已验证：ACR 固定摘要镜像、`unimap_config` 可写卷、`0600` 配置、应用原子保存、
容器重启哈希一致、业务备份、受限配置备份以及“临时改为 auto → 原子恢复 → 重启回到 cdp”。
不可写页面提示属于后续 UX 增强，不再阻断当前可写生产基线。

### RW-02 Quake/Hunter 生产配置与真实 CDP 验收

状态：**生产配置已完成，真实 CDP 验收待执行**。

`configs/config.prod.yaml` 已显式启用 Quake/Hunter，使用正确 Base URL、QPS 1、30 秒超时；
Compose 已透传可选 `QUAKE_API_KEY`、`HUNTER_API_KEY`。Key 留空时只表示允许进入 Web-only
路径，不代表登录态或 CDP 采集已通过。

逐引擎完成：

1. API 最小查询；
2. Cookie 登录状态；
3. CDP 打开真实结果页；
4. 非空结构化资产；
5. 结果页 PNG 人工检查；
6. SQLite 合并持久化；
7. 图片通知；
8. 容器重启后会话和结果恢复。

### RW-03 五个稳定引擎 CDP 能力定级

现有 CDP URL、Cookie、Network/DOM 和 `collect_and_capture` 代码不能替代真实验证。

完成标准：

- FOFA、Hunter、ZoomEye、Quake、Shodan 分别记录通过、受限或不支持；
- 登录页、验证码页、空资产不能报告成功；
- 页面改版导致的 selector/network 失败有明确诊断；
- 形成独立于 Bridge 的 CDP live E2E 和日期化证据。

### RW-04 Censys/DayDayMap 浏览器能力与 UI 决策

状态：**能力误报已修复，当前产品范围维持 API-only；浏览器能力仍未实现**。

先确定产品范围：

- 若保持 API-only：删除或明确隔离不可达的 Web-only/选择器占位，API/UI/文档返回一致的能力说明；
- 若要支持浏览器：补齐查询 URL、UQL Web 翻译、登录态、Cookie、Bridge/CDP 结构化采集、截图、前端选择、设置页、调度、通知和 live E2E。

两者不能仅因“选择器存在”标记为 Bridge/CDP 已支持。

缺少完整 API 凭据时现在会跳过注册并记录警告，readiness 和引擎列表不再把不可执行的
Censys/DayDayMap Web-only 后端计为可用。后续若选择浏览器路线，仍需完成上述整条链路。

### RW-05 Cookie/Profile 无人值守闭环

- 周期性打开真实结果页验证登录；
- 连续失败熔断对应浏览器任务；
- 区分 Cookie 缺失、过期、登录墙、验证码、页面改版和网络失败；
- 通知最后成功时间与恢复操作；
- 更新 Cookie 后自动执行低额度恢复验收。

## 3. P2：应当收尾

### RW-06 设置页 Base URL 纠正

状态：**已完成（2026-07-23）**。

- Hunter 占位值已与默认 `https://hunter.qianxin.com` 一致；
- Quake 占位值已与默认 `https://quake.360.net/api` 一致；
- 已增加前端契约测试，防止再次偏离 adapter 默认值。

### RW-07 配额功能

当前明确未实现：

- 配额趋势图；
- 自动刷新设置；
- 阈值告警。

入口继续保持禁用，直到服务端存储、调度、告警和 UI 全部完成。

### RW-08 发布与工作区清理

- 状态：**进行中（2026-07-24 已完成临时工具迁移和文档保留）**。
- 审查并提交本轮文档；
- `CLAUDE.md` 已按用户要求保留并同步当前事实；**已完成**
- 根目录 `tmp_speedtest.go` 已迁移为带 `manual_browser_test` 标签的
  `tools/chrome-speedtest` 手动工具；**已完成**
- 推送 `develop` 当前领先远端的提交；
- 发布前再次执行 `go test -race ./...`、`go vet ./...`、`go build ./...` 和 GUI build tag。**已完成**

### RW-09 网页巡检确定性收尾

状态：**部分完成（2026-07-24），安全证据截图与云端真实闭环待完成**。

- 手动巡检、定时巡检与基线刷新读取同一套实时 Tamper 配置和浏览器 allocator；
- 基线刷新按实际 `saved` 结果统计，不再把业务失败计为成功；
- 历史列表和导出支持 `start_time`、`end_time`，`count` 返回过滤后总数；
- “发现变化 → 证据截图 → 图片通知”未启用：当前调用前 URL 检查不能阻止浏览器后续跨主机
  重定向或 DNS rebinding，必须先在 CDP/Extension 浏览器层形成逐跳/连接级 SSRF 防护；
- 尚需在云端用受控页面完成基线、真实变化、历史、飞书图片送达、恢复和重启后复验。

## 4. P3：可选增强

- ~~历史 API 增加 `start_time`、`end_time`；~~ **已完成（2026-07-24）**
- 提供受控恢复命令或恢复 API，而不只提供备份创建/列表；
- 完成配额趋势与告警后增加长期数据保留策略；
- ARM64 镜像与 Chromium 验收。

## 5. 本轮已纠正的记录

- 不再把 Censys、DayDayMap 描述成已完成 Extension/Bridge 能力；
- 不再把历史 Bridge 抓取测试当作 CDP 证据；
- 将备份任务改为已提交的第 23 种任务；
- 生产运行配置改为首次初始化的可写命名卷；
- 未配置引擎不再被默认强制启用，Censys/DayDayMap 缺凭据时不再伪注册 Web-only；
- Quake/Hunter 生产模板和设置页 Base URL 已统一；
- 当前权威文档统一使用本文件的支持矩阵和待办编号。
- 巡检运行时配置、基线 allocator、时间筛选和真实总数已完成代码收口；变化证据截图因浏览器层
  SSRF 防护不足继续保留待办。

## 6. 本地验证（2026-07-24）

- `go test -race ./...`：通过；
- `go vet ./...`：通过；
- `go build ./...`：通过；
- `go build -tags gui ./cmd/unimap-gui`：通过；
- `go build -tags manual_browser_test ./tools/chrome-speedtest`：仅编译通过，未运行 Chrome；
- `node --check web/static/js/main.js`：通过；
- `git diff --check`：通过。

2026-07-23 的 `govulncheck ./...`、入口脚本语法和阿里云容器证据继续有效，但本轮没有重复
执行真实云端页面变化与飞书图片送达；这两项仍是 RW-09 的外部验收边界。
