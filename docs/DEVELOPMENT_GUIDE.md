# UniMap 开发规范

> 最后按代码核对：2026-07-13。Go 版本以 `go.mod` 中的 1.26.5 为准。

## 目录与边界

```text
cmd/                    # CLI、Web、GUI 入口
internal/
  adapter/              # 搜索引擎适配器与编排
  config/               # 配置与热加载
  core/unimap/          # UQL 解析和归并
  plugin/               # 进程内插件与处理管道
  scheduler/            # 调度器与 Runner
  screenshot/           # CDP、Bridge、截图路由与批次持久化
  service/              # 应用服务层
  tamper/               # 网页巡检
  utils/                # 通用工具（没有 internal/util）
web/                    # 路由、handler、中间件、模板和静态资源
configs/                # 配置模板与本地配置
docs/                   # 当前文档、决策记录与归档资料
```

`internal` 包不能被仓库外模块导入。外部可复用能力需通过公开模块、HTTP API 或另行设计的 SDK 暴露，不要在文档中建议外部插件直接导入 `internal/service`。

## Go 规范

- 执行 `gofmt`；变更导入时使用 `goimports`。
- 遵循 Go 命名习惯：导出标识符用 MixedCaps，非导出标识符小写 MixedCaps；不要使用“全大写下划线”常量规则。
- 错误需添加上下文并用 `%w` 包装；不得把内部路径、令牌或原始凭证返回给客户端。
- 优先接受小接口、返回具体结构体；并发路径必须可在 `-race` 下验证。
- HTTP 路由只在 `web/router.go` 注册。新增 API 必须是 `/api/v1/...`，并同步更新 [API.md](API.md) 与 handler 测试。

## 测试与验证

开发中至少运行受影响包的竞态测试，提交前运行：

```bash
go test -race ./...
go vet ./...
go build ./...
```

GUI 单独验证：

```bash
go test -tags gui ./cmd/unimap-gui
go build -tags gui ./cmd/unimap-gui
```

测试采用 table-driven 与 Arrange-Act-Assert 风格。任何安全回归（认证、CSRF、SSRF、路径遍历、Bridge token）都应添加最小的正反用例。

## 安全与配置

- 不提交 API Key、Cookie、管理令牌、Bridge token、Webhook URL 或真实测试资产。
- `${VAR}` / `$VAR` 是显式环境变量占位符；直接配置值不会被同名环境变量自动覆盖。
- 启用 Web 认证、限流和生产 TLS/反向代理配置后，再将服务绑定到非 loopback 地址。
- 外部 URL 功能必须沿用已有 SSRF 防护，不得添加绕过路径。

## 文档规则

- 当前操作文档只描述当前代码契约；历史事实、审计结果和计划应带日期与“历史快照/已归档”标识。
- 修改路由、任务类型、引擎范围、构建要求或配置优先级时，同时更新 README、API、Runbook 和相关 ADR/memory 索引。
- 文档示例必须可运行；特别是 GUI 命令必须带 `-tags gui`，API 示例必须带 `/api/v1` 前缀。

## 构建与部署

```bash
go build -o output/unimap-web ./cmd/unimap-web
go build -o output/unimap-cli ./cmd/unimap-cli
go build -tags gui -o output/unimap-gui ./cmd/unimap-gui
```

Docker 与本机部署均使用 `configs/config.yaml` 或部署环境注入的变量。部署运行方式以仓库中的 Docker 文件和脚本为准；修改部署流程时必须同步更新 [RUNBOOK.md](RUNBOOK.md)。
