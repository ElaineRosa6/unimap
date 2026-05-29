# 浏览器查询降级实施计划

## 1. 目标

在不破坏现有 API 查询、截图、定时任务和 CLI 行为的前提下，为免费用户提供可用的浏览器查询线路。

核心判断：

- API adapter 继续作为主查询路径。
- 浏览器查询作为补充路径，不全局替代 API。
- 免费用户通常只有 Hunter 和 Quake 能通过 API 返回结果。
- FOFA、ZoomEye、Shodan 在免费或未购买 API 能力时，需要通过 CDP 或 Extension 访问 Web 端并采集 DOM。

## 2. 设计原则

1. **默认不改变老行为**
   - 已配置 API Key 的引擎继续优先走 API。
   - API 成功时不触发浏览器采集。
   - 自动 API 失败降级默认关闭，避免影响现有用户和定时任务。

2. **浏览器路径显式可控**
   - 用户主动开启浏览器查询时才执行打开、采集或截图。
   - Web-only adapter 可以使用浏览器 backend。
   - API 失败后的自动 fallback 需要配置开关和引擎白名单。

3. **按引擎逐步启用**
   - 第一阶段覆盖 Web-only adapter 和显式浏览器查询。
   - 第二阶段只对 FOFA、ZoomEye、Shodan 开放可选 fallback。
   - Hunter、Quake 默认保持 API 优先，不自动走浏览器 fallback。

4. **失败隔离**
   - 浏览器采集失败不得吞掉原 API 错误。
   - 浏览器 runtime 不可用时，应快速返回清晰错误，不阻塞整个查询。
   - DOM 解析失败只影响对应引擎，不影响其他引擎结果。

## 3. 推荐配置

后续建议新增独立查询配置，避免复用截图字段承载查询语义：

```yaml
query:
  browser_fallback:
    enabled: false
    on_api_error: false
    on_empty_result: false
    engines:
      - fofa
      - zoomeye
      - shodan
```

字段语义：

| 字段 | 默认值 | 说明 |
|------|--------|------|
| `enabled` | `false` | 是否允许 API 失败后自动尝试浏览器采集 |
| `on_api_error` | `false` | API 返回错误时是否 fallback |
| `on_empty_result` | `false` | API 返回空结果时是否 fallback |
| `engines` | `fofa,zoomeye,shodan` | 允许自动 fallback 的引擎白名单 |

显式浏览器查询不受 `query.browser_fallback.enabled` 限制。用户手动选择浏览器查询时，仍应走当前浏览器 runtime。

## 4. 分阶段实施

### 阶段一：接通现有浏览器 backend

目标：保证 Web-only adapter 和显式浏览器查询在 CDP、Extension、Auto 三种模式下都能工作。

任务：

- 在 Web 服务初始化时，为 Web-only adapter 注入当前可用的浏览器 provider。
- `mode: auto` 使用 `ScreenshotRouter`。
- `mode: extension` 使用 `ExtensionProvider`。
- `mode: cdp` 使用 `CDPProvider`。
- 显式浏览器查询也使用同一套 provider 选择逻辑。

兼容性要求：

- 不改变 API adapter 的查询优先级。
- 不改变 CLI 直接查询行为。
- 不改变目标站截图、批量截图和定时截图行为。

当前状态：

- 已完成基础接通：`web/server.go` 和 `web/query_handlers.go` 已让浏览器查询接入当前可用 provider。

### 阶段二：增加配置开关和查询层策略

目标：为 API 失败后的自动 fallback 提供受控开关，但默认关闭。

任务：

- 在 `config.Config` 增加 `query.browser_fallback` 配置。
- 在 `applyDefaults` 中设置安全默认值。
- 在 `validate` 中校验引擎白名单、布尔开关和字段组合。
- 在设置页或运维文档中说明默认不开启自动 fallback。

兼容性要求：

- 配置缺省时行为与当前版本一致。
- 老配置文件不需要迁移即可启动。
- 配置保存时不得覆盖用户已有 engine、screenshot、web 配置。

### 阶段三：实现 API 失败后的可选 fallback

目标：当 API adapter 失败，并且配置允许时，调用浏览器 backend 补采结果。

建议位置：

- 优先在 `UnifiedService.Query` 或 `EngineOrchestrator` 附近处理。
- 保持 adapter 接口稳定，避免每个 adapter 内部重复实现 fallback。

触发条件：

- 当前引擎在白名单内。
- API adapter 返回错误，且 `on_api_error=true`。
- 或 API 返回空结果，且 `on_empty_result=true`。
- 当前存在可用浏览器 provider。

结果处理：

- 浏览器结果进入统一 `UnifiedAsset` 流程。
- 保留 API 错误，但如果浏览器成功返回结果，前端可以降低错误展示优先级。
- 资产增加来源标记，便于排查：

```go
asset.Extra["collection_method"] = "browser"
asset.Extra["browser_runtime"] = "cdp" // or "extension" / "auto"
```

兼容性要求：

- API 成功时不得触发 fallback。
- fallback 失败时不得覆盖原 API 错误。
- 多引擎查询中，一个引擎 fallback 失败不得影响其他引擎。

### 阶段四：前端和用户提示收敛

目标：让用户明确当前结果来自 API 还是浏览器采集。

任务：

- 查询结果展示增加来源标识：`API`、`浏览器采集`、`API 失败后浏览器补采`。
- 浏览器查询按钮文案与行为统一：
  - `open`：只打开页面。
  - `collect`：采集结构化结果。
  - `collect_and_capture`：采集结构化结果并截图。
- 对 CDP 和 Extension 不可用状态给出明确提示。

兼容性要求：

- 保留旧 `browser_action` 的兼容映射。
- 旧前端请求仍能执行，不因 action 命名调整直接失败。

### 阶段五：观测和逐步默认化

目标：根据真实使用情况决定是否扩大 fallback 默认范围。

建议先观测：

- 每个引擎 API 成功率。
- 每个引擎浏览器采集成功率。
- DOM 解析失败率。
- 登录墙、验证码、风控错误数量。
- CDP / Extension runtime 可用率。

默认化条件：

- 某引擎浏览器采集成功率稳定。
- DOM selector 维护成本可接受。
- 失败不会显著拖慢查询。
- 日志和 UI 能清楚解释结果来源。

## 5. 健康检查要求

浏览器 fallback 前必须确认 runtime 可用：

| Runtime | 必须检查 | 不足之处 |
|---------|----------|----------|
| CDP | remote debug URL 在线，或本地 Chrome 可启动 | 当前健康检查还需要区分“可启动”和“不存在” |
| Extension | Bridge service 运行，存在 live client，最近有任务拉取或回调 | 只看 BridgeService 启动不够 |
| Auto | 根据任务语义选择 provider，并允许受控 fallback | 当前 Auto 仍偏全局模式选择 |

短期策略：

- CDP 不在线时，不自动假设可用。
- Extension 没有 live client 时，不触发自动 fallback。
- 显式浏览器查询可以返回可操作错误，引导用户连接 CDP 或扩展。

## 6. 测试矩阵

最低测试覆盖：

| 场景 | 期望 |
|------|------|
| API adapter 成功 | 不触发浏览器 fallback |
| API adapter 失败，fallback 关闭 | 行为与旧版本一致 |
| API adapter 失败，fallback 开启，引擎在白名单 | 调用浏览器 backend |
| API adapter 失败，fallback 开启，引擎不在白名单 | 不调用浏览器 backend |
| Web-only adapter，无浏览器 provider | 返回清晰错误 |
| Web-only adapter，CDP provider 可用 | 返回浏览器采集结果 |
| Web-only adapter，Extension provider 可用 | 返回浏览器采集结果 |
| Auto mode | 通过 router 选择 provider |
| 浏览器采集失败 | 保留原错误，不影响其他引擎 |
| 浏览器采集成功 | 结果进入统一去重、合并、导出流程 |

建议测试命令：

```powershell
go test ./web ./internal/service ./internal/adapter ./internal/screenshot
```

涉及配置结构时再追加：

```powershell
go test ./internal/config ./cmd/unimap-cli
```

## 7. 回滚策略

任意阶段都应可快速回滚到 API-only 行为。

回滚方式：

1. 将 `query.browser_fallback.enabled` 设为 `false`。
2. 保留显式浏览器查询按钮，但不自动 fallback。
3. 如果浏览器 provider 接入引发问题，可临时只在 `mode: auto` 注入 Web-only backend。
4. 如果 DOM selector 大面积失效，禁用对应引擎的 browser fallback 白名单。

必须保留：

- API adapter 查询路径。
- CLI 查询路径。
- 定时任务查询路径。
- 截图 provider 原有能力。

## 8. 风险清单

| 风险 | 影响 | 缓解 |
|------|------|------|
| DOM 改版 | 浏览器采集失败或字段缺失 | selector 单独维护，失败隔离 |
| 登录墙或验证码 | 浏览器采集不可用 | 返回 login_required，提示用户登录 |
| Extension 离线 | 桌面采集失败 | 检查 live client，允许 CDP fallback |
| CDP 不可启动 | 服务端采集失败 | 健康检查区分 Chrome 不存在和 remote 离线 |
| 结果重复 | API 和浏览器结果重复 | 统一走 ResultMerger |
| 查询变慢 | 用户等待时间增加 | fallback 默认关闭，设置超时 |
| 错误被吞 | 难以排查 | 保留 API 错误和 browser 错误 |

## 9. 完成标准

阶段性完成标准：

- Web-only adapter 在 CDP、Extension、Auto 模式均能拿到 provider。
- 显式浏览器查询不依赖单一 auto router。
- API 成功路径不受影响。
- 自动 fallback 有配置开关、默认关闭、有白名单。
- 测试覆盖 API 成功、API 失败、fallback 开关、provider 不可用、采集失败和结果合并。
- 文档说明免费用户路径、配置方式、风险和回滚方式。
