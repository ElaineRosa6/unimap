# CI 收尾执行清单（2026-06-26）

> 目标：闭环当前已识别的 CI 失败项（Lint/Test/Security），并提供可直接执行的验证与回写步骤。
>
> 适用分支：`develop`

---

## 1. 执行范围

本清单只覆盖三类确认未闭环项：

1. `go test -race` 与 `CGO_ENABLED=0` 冲突。
2. `govulncheck` 告警依赖版本落后。
3. 行尾符不一致（CRLF）导致的格式检查失败。

---

## 2. 修复步骤（按顺序）

### Step 1：修复 CI race 环境冲突

**目标**：让测试作业可稳定执行 `go test -race`。

**修改建议**：

- 在 `.github/workflows/ci.yml` 保留全局 `CGO_ENABLED=0` 给 build/lint/security。
- 对 `test` job（或 `Run tests (with race detector)` step）显式覆盖 `CGO_ENABLED=1`。

**验收标准**：

- CI 中 `test` job 不再因 race/cgo 冲突失败。

---

### Step 2：升级漏洞相关依赖并整理模块

**目标**：消除当前安全扫描中的已知依赖告警。

在仓库根目录执行：

```bash
go get golang.org/x/image@v0.43.0 golang.org/x/net@v0.55.0 github.com/redis/go-redis/v9@v9.6.3
go mod tidy
```

**验收标准**：

- `go.mod`/`go.sum` 已更新。
- CI `Security Scan` 中 `govulncheck` 不再报这三项旧版本告警。

---

### Step 3：统一 LF 行尾并重新格式化

**目标**：消除 CRLF 导致的格式校验失败。

**修改建议**：

- 新增 `.gitattributes`，统一文本文件行尾为 LF（至少覆盖 `*.go`, `*.md`, `*.yml`, `*.yaml`, `*.js`, `*.html`, `*.css`）。

在仓库根目录执行：

```bash
git add --renormalize .
gofmt -w .
```

**验收标准**：

- `gofmt -l .` 返回空。
- CI `Lint` 中格式检查通过。

---

### Step 4：本地回归验证（推送前必做）

在仓库根目录执行：

```bash
go build ./...
go test -race -count=1 ./...
go test -coverprofile=coverage.out -covermode=atomic ./...
go vet ./...
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...
```

**验收标准**：

- 上述命令全部通过，再推送触发 CI。
- 若 `govulncheck` 因本机 Go 1.26.0/1.26.2/1.26.3 标准库漏洞基线报错，需要用补丁版 toolchain 复核（建议 Go 1.26.4+）。

---

### Step 5：文档与记忆回写

**目标**：避免“代码已修但状态文档仍未完成”的信息漂移。

建议回写位置：

- `CLAUDE.md` 的“剩余工作汇总”区块（CI 3 项状态）。
- `memory/MEMORY.md` 对应“当前活跃/当前文档”索引。

**验收标准**：

- 文档状态与最新 CI 实际状态一致，附最近一次有效 run 或 commit 引用。

---

## 3. 当前状态（2026-06-26）

- `.github/workflows/ci.yml` 已为 race 测试步显式设置 `CGO_ENABLED=1`。
- `.gitattributes` 已新增并执行 `git add --renormalize .`，仓库文本行尾统一为 LF。
- `go.mod` / `go.sum` 已升级到 `golang.org/x/image@v0.43.0`、`golang.org/x/net@v0.55.0`、`github.com/redis/go-redis/v9@v9.6.3`。
- 本地 `go build ./...`、`go test -race -count=1 ./...`、`go vet ./...` 已通过。
- `govulncheck` 在当前本机 toolchain 上仍受 Go 1.26.0/1.26.2/1.26.3 标准库漏洞基线影响，CI 侧建议确认补丁版 Go toolchain。

## 4. 最小交付清单（PR 维度）

建议将一次收尾拆成一个最小 PR，包含：

1. `ci.yml` 的 race 环境修复。
2. `go.mod`/`go.sum` 依赖升级。
3. `.gitattributes` + 行尾归一化结果。
4. `CLAUDE.md` + `memory/MEMORY.md` 状态回写。

这样可以保证“问题修复、验证证据、状态同步”在同一次变更里闭环。
