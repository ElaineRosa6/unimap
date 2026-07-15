# UniMap 逻辑可用性三项完善闭环（2026-07-15）

## 核实结论

1. Alert JSON crash-safe 生产实现已在更早提交中采用固定 `.tmp` 文件写入后 `os.Rename`；本轮没有重复改生产逻辑，而是补充临时文件不残留的回归断言并清理过期待办。
2. ICP history 的 `keyword` 当时仍使用 SQL 等值查询，部分关键词会返回空结果。
3. Scheduler 创建/更新只校验 payload 大小与 webhook；必填字段到 Runner 执行期才报错。数组字符串和字段别名在不同路径上的兼容行为不一致。

## 当前行为

- Alert records：配置持久化路径后，以 `0600` 写入 `.tmp`，再原子替换目标 JSON；失败继续只记录 warning。
- `GET /api/v1/icp/history`：`keyword` 使用转义后的包含匹配，用户输入的 `%`、`_` 不会被当作 SQL 通配符；`type` 和 `task_id` 仍精确匹配。
- Scheduler payload：
  - 创建、更新时根据任务类型检查必填字段，错误包含 Runner 类型和字段名。
  - 字符串数组可使用 JSON 数组或逗号分隔字符串。
  - `urls` 兼容 `targets`，`engines` 兼容 `engine`。
  - 推荐字段放在 payload 顶层；旧任务的 `extra.query` 等既有读取路径继续兼容。
  - Runner 执行期缺字段错误也包含任务类型。

## 提交与验证

- `4278a3f test: verify atomic alert persistence`
- `a982189 feat: support partial ICP history keywords`
- `1a32f46 feat: clarify scheduler payload contracts`
- 推送目标：`origin/develop`
- 验证：`go test -race ./...` 通过。

## 文档口径

- HTTP 契约以 [`API.md`](API.md) 的“调度任务 Payload 契约”和 ICP history 条目为准。
- [`E2E_FULL_FLOW_TEST_2026-07-12.md`](E2E_FULL_FLOW_TEST_2026-07-12.md) 保留当日精确匹配与严格 payload 的历史事实，并追加 2026-07-15 勘误，不能再作为当前限制引用。
- [`AUDIT_REMEDIATION_GUIDE.md`](AUDIT_REMEDIATION_GUIDE.md) 已将 Alert 原子持久化标记为完成并移除对应行动项。
