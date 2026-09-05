# 项目检查与优化（2026-09-06，持续进行）

## 范围与基线

- 基线提交：`d20323f`，不是旧状态页记录的 `e31e03d`。
- 本轮覆盖历史库数据完整性和 CI 发布依赖；这不是整个项目已无问题的结论。
- 基线 CI：[33855935603](https://github.com/ElaineRosa6/unimap/actions/runs/33855935603) 成功。该纯文档提交未触发按路径过滤的 bridge-smoke。
- 不更动运行数据库、配置凭据、云端任务开关；不自动清理历史遗留数据。

## 已复现与本地修复

### P1：删除操作历史遗留结果明细

`operation_results` 定义了 `ON DELETE CASCADE`，但 `NewDatabase` 没有为连接启用外键。新回归测试在原代码上证实：单条删除、按类型清空、全部清空后明细仍存在；连接池的三个独立连接和数据库重开后均接受不存在父记录的明细。

修复：DSN 添加 `_foreign_keys=on`，使每个新建连接都执行外键约束，而不是仅在一个连接上执行 PRAGMA。现有数据库表结构保持不变。

回归覆盖：上述三种删除方式、保留其他类型的记录、连接池和重开后的外键开关、拒绝新孤儿数据。历史库与配置包竞态测试已通过。

已有孤儿行不会因为启用外键自动消失。上线前可在备份副本执行以下只读查询评估遗留数量：

```sql
SELECT COUNT(*) AS orphan_results
FROM operation_results AS r
WHERE NOT EXISTS (SELECT 1 FROM operation_history AS h WHERE h.id = r.history_id);
```

清理遗留数据应单独确认并备份；删除行也不等同于数据库文件立即缩小。

### P2：镜像发布没有等待所有验证门槛

Docker job 原先只依赖 test/lint/security，浏览器 E2E 或扩展脚本检查失败仍可能发布镜像。现补上 `headless-browser` 和 `extension-scripts` 依赖，并添加解析 YAML 的回归测试。

### P2：多平台覆盖率附件使用同一名称

矩阵中两个系统都上传名为 `coverage` 的附件，存在同名冲突风险。改为 `coverage-${{ matrix.os }}`；测试约束附件名称包含矩阵维度。[官方说明](https://github.com/actions/upload-artifact#not-uploading-to-the-same-artifact)。不把这一潜在风险描述为基线 CI 实际失败。

## 后续检查队列

### 第二轮：手工保存历史的原子性

故障注入在第二条结果插入时触发 SQLite ABORT。旧 HTTP handler 返回 500，但留下 1 条主记录；重试成功后主记录变成 2 条。改用现有 `CreateHistoryWithResults`，失败时整次事务回滚；成功重试只有 1 条历史、2 条明细。补测空结果、普通结果和默认 1000 条截断上限。接口字段不变，不扩展为成功请求幂等协议。

- 历史 repository 部分读取方法未检查 `rows.Err()`：需注入迭代错误证明影响，再修复。
- 截图批次文件操作的符号链接与路径边界：需受控本地回归验证。
- 查询/调度取消、资源释放、配置只读提示和性能热点继续审查。
- 浏览器与生产验收维持既有边界，不用本地单测替代真实 CollectAndCapture 验收。
- 本轮 CI 修改的 GitHub 实跑尚待提交后验证；本地配置测试只证明结构约束。
