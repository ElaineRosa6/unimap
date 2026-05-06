# UniMap 变更日志

---

## [2026-04-29] 第三轮安全修复 — 24 项 Code Review 剩余问题修复

> **分支**: `release/major-upgrade-vNEXT`
> **变更类型**: 安全修复 + 代码质量提升
> **涉及模块**: scheduler、auth、config、backup、distributed、web、cmd/unimap-cli

### 一、Webhook SSRF 防护（DNS/重定向绕过修复）

- **DNS 解析校验** — `ValidateWebhookURLPublic` 增加 `net.LookupIP` 校验所有解析 IP，阻止私有/回环地址 (`internal/scheduler/scheduler.go`)
- **安全 HTTP Client** — `safeWebhookClient()` 拒绝所有重定向 + 自定义 DialContext 阻止连接私有 IP (`internal/scheduler/scheduler.go`)

### 二、API Key 认证体系修复

- **SHA-256 hash 比对** — 引入 `KeyHash` 字段，`ValidateAPIKey`/`UpdateLastUsed` 改用 hash 比对，解决持久化后密钥不可验证问题 (`internal/auth/api_key.go`)
- **统一 map key 策略** — `GenerateAPIKey` 改用 ID 为 map key，`loadFromStorage` 使用 ID 保持一致 (`internal/auth/api_key.go`)
- **expiresIn=0 永不过期** — `zeroOrExpiry()` 将 0 转为 `IsZero` 时间 (`internal/auth/api_key.go`)
- **auth 平行认证接入** — Server 结构体新增 `apiAuth` 字段；路由层接入 `OptionalAPIKey` 中间件，API Key 认证体系正式接入主路由 (`web/server.go`, `web/router.go`)

### 三、分布式任务队列持久化

- **JSON 快照持久化** — 新增 `NewTaskQueueWithPath()` + `saveLocked()`/`loadSnapshot()`，所有状态变更（Enqueue/Claim/SubmitResult/Delete）自动保存 (`internal/distributed/task_queue.go`)
- **Server 接入** — TaskQueue 初始化传入 `./data/distributed_tasks.json` 路径 (`web/server.go`)

### 四、备份系统错误处理

- **Source 失败累积错误** — 静默跳过的 source 错误累积并在无有效文件时返回错误 (`internal/backup/backup.go`)
- **Tar 写入失败部分成功** — 累积 tar 错误，最终返回时包含失败统计 (`internal/backup/backup.go`)
- **BackupConfig 注释修正** — 注释改为"绝对路径或相对路径" (`internal/backup/backup.go`)

### 五、限流与安全头

- **X-Real-IP 代理检查** — 和 XFF 一样加入 `isPrivateOrInternalHost` 检查，防止可伪造代理头 (`web/middleware_ratelimit.go`)
- **CORS 默认 X-Admin-Token** — config 层默认 AllowedHeaders 添加 `X-Admin-Token` (`internal/config/config.go`)

### 六、可用性与安全性

- **Admin Token 非loopback 提示** — 非回环绑定打印 token 并提示保存 (`internal/config/config.go`)
- **CLI JSON 防覆盖** — `writeJSONFile` 改用 `O_EXCL`，与 CSV 一致 (`cmd/unimap-cli/api_subcommands.go`)

### 七、测试修复

- **TestGenerateAPIKey/zero_expiration** — `expiresIn=0` 返回 `IsZero` 时间，测试逻辑更新
- **TestUpdateLastUsed** — 改为按 ID 查找验证（map key 策略变更）
- **config 测试** — 默认 CORS headers 预期更新

### 验证结果

- `go build ./...` — 构建成功
- `go test -race ./...` — 全部通过（0 failures, 0 races）

---
