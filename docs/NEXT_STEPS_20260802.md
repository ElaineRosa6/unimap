# UniMap V1 后续实施清单

> 核对日期：2026-08-02（Asia/Shanghai）
> 当前分支：`develop`，存在尚未推送到 `origin/develop` 的本地提交；实施前用 `git rev-list --count origin/develop..HEAD` 重新确认数量。
> 定位：面向人工操作的 Web 平台，负责 UQL、多引擎查询、CDP/Extension、截图、巡检、调度、通知和持久化。

## 1. 已确认基线

- 工作区干净；`go test ./...`、`go vet ./...`、`go build ./cmd/...` 已于 2026-08-02 退出 0。
- GUI 已删除，正式入口为 `unimap-web` 与 `unimap-cli`。
- 七引擎已有 API 适配；七引擎 CDP L1/L3、搜索 URL 和 Extension 选择器代码已完成。
- Quake/Hunter 有真实 CDP 证据；Quake L1 已按当前 endpoint 校准。
- 2026-08-02 重载 Extension 0.4.2 后，V1 Bridge 状态为在线，Quake `port="80"` 浏览器采集得到 1 组、10 条结构化资产，浏览器错误数为 0；验证未导出或回显 Cookie。
- 云端安全验收 fixture、会话熔断、失败分类、恢复提示和共享 SessionHealthTracker 已实现。

## 2. 当前必须完成

### V1-01 闭合 Quake Web/Router 主链路

本轮真实结果：Browser/Bridge 通道成功返回 10 条资产，但 `/api/v1/query` 外层仍返回 HTTP 502，原因是普通 API 通道没有可执行的 Quake adapter。浏览器成功结果不应被普通 API 失败整体覆盖。

实施：

1. 明确 API 与 Browser 并行结果的成功语义；
2. API 失败但 Browser 返回非空资产时，响应标记 `partial` 或成功，并保留脱敏 API 错误；
3. Browser 也失败时才返回整体失败；
4. 为“API adapter 缺失 + Browser 成功”“API 失败 + Browser 成功”“两路均失败”增加 handler/service 回归测试；
5. 使用当前 Quake 登录态真实运行 `collect_and_capture`。

完成标准：

- HTTP 响应包含 10 条真实 Browser 资产且不返回整体 502；
- 生成真实 PNG，文件可解码且 MIME/扩展名一致；
- SQLite 只有一条合并查询历史，资产来源可区分；
- Router 状态、错误数组和持久化状态与实际一致。

### V1-02 五个稳定引擎 CDP 定级

当前结论：

| 引擎 | 当前真实证据 | 剩余工作 |
|---|---|---|
| Quake | CDP 非空通过；Extension collect 非空通过 | 完整 `collect_and_capture`、SQLite、通知、重启恢复 |
| Hunter | 历史 CDP 非空通过，API 通道通过 | 固化独立 live test，复验当前页面选择器与截图 |
| FOFA | API 非空通过；历史 Bridge 通过 | 当前账号 CDP L1/L3 与 PNG 实测 |
| ZoomEye | API 认证/配额通过，搜索积分受限 | 记录受限结论；有会员后再做 CDP 非空验收 |
| Shodan | Key 有效但非会员搜索受限 | 记录受限结论；有会员后再做 CDP/Bridge 非空验收 |

实施要求：每个引擎必须明确标为“通过”“受限”“不支持”或“未配置”；登录页、验证码页和空资产不算非空通过。为 FOFA、Hunter、ZoomEye、Shodan 增加与 Quake 同级的显式 live E2E 入口。

### V1-03 Cookie/Profile 无人值守闭环

实施：

1. 周期性打开真实结果页验证登录态；
2. 设置页保存 Cookie 后立即调度一次低额度 LoginStatusCheck；
3. 熔断打开时阻止浏览器业务任务、保留恢复探针；
4. 登录恢复后自动半开验证并关闭熔断；
5. 对 `cookie_expired`、`login_wall`、`captcha`、`page_changed`、`network` 分别发送可执行告警；
6. 容器重启后验证 Profile、熔断状态和任务结果恢复。

完成标准：在真实 Quake/Hunter 会话中演示“健康 -> 失效 -> 熔断 -> 更新会话 -> 自动复验 -> 恢复”。

### V1-04 云端巡检与安全闭环

使用 `tools/acceptance-fixture/` 完成：

1. DNS rebinding 在连接私网 sink 前失败；
2. 建立页面基线并触发受控真实变化；
3. 生成变化证据 PNG；
4. 历史记录、截图路径和变化摘要一致；
5. 飞书/Webhook 收到文字与图片；
6. 容器重启后基线、历史和配置恢复；
7. 验收完成前保持自动篡改证据截图 gate 关闭。

### V1-05 发布与远端同步

现状：本地 `develop` 领先 `origin/develop`，具体数量以实施时的 Git 命令输出为准。

最终门禁：

```powershell
go test -race ./...
go vet ./...
go build ./cmd/...
node --check web/static/js/main.js
node --test tools/extension-screenshot/test/capture_quake.test.mjs
node --test tools/extension-screenshot/test/tab_url.test.mjs
govulncheck ./...
git diff --check
```

完成标准：门禁通过、权威文档一致、真实验收边界明确；确认远端和发布窗口后再推送 `develop`。

## 3. 产品决策

### Censys / DayDayMap

代码已具备 API、CDP L1/L3、搜索 URL 和 Extension 选择器，但没有稳定 Web UI 与真实浏览器验收。二选一：

1. **保持 API-only（建议先选）**：UI、能力接口和文档明确显示 API-only，不把浏览器代码宣称为已验收；
2. **升级为完整浏览器引擎**：补登录态、Cookie、Web UI、设置、调度、通知、Bridge/CDP live E2E 和真实截图。

在五个稳定引擎与云端巡检闭环完成前，不把这两项作为主攻方向。

## 4. 后续增强

| 编号 | 工作 | 优先级 |
|---|---|---|
| V1-06 | 配额趋势、自动刷新、阈值告警 | P2 |
| V1-07 | 受控恢复命令或恢复 API | P2 |
| V1-08 | ARM64 镜像与 Chromium 验收 | P3 |
| V1-09 | Censys/DayDayMap 完整 Web 接入 | P3，取决于产品决策 |

## 5. 推荐实施顺序

1. **V1-01**：先修 Quake Browser 成功但外层 502，形成一条真正可用的主链路；
2. **V1-03**：利用当前有效 Quake 会话完成无人值守闭环；
3. **V1-04**：部署 fixture，完成云端巡检、通知和重启验收；
4. **V1-02**：按账号条件完成或明确限制五引擎 CDP 定级；
5. **V1-05**：最终门禁并发布；
6. 后续再做 V1-06 至 V1-09。

## 6. 协作输入

- V1-01 和单元/本地 E2E 不需要新增材料；
- V1-04 需要一台受控云机、测试域名/DNS 控制权限以及飞书或 Webhook 测试目标；
- FOFA/Hunter 的当前浏览器定级需要对应登录态；Shodan/ZoomEye 无会员时维持“预期受限”；
- 推送本地提交前需确认远端与发布窗口。
