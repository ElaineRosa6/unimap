# UniMap 插件系统架构

> 最后按代码核对：2026-07-13。插件系统是**进程内、源码级注册**机制；不提供从外部目录扫描 `.so`、热加载或跨模块 SDK。

## 边界

- 插件接口位于 `internal/plugin`，配置与数据类型位于 `internal/model`。
- Go 的 `internal` 规则禁止仓库外模块导入这些包。因此“第三方独立仓库编译插件并动态加载”的模式不受支持。
- `PluginManager` 可以在同一进程内加载、启动、停止、卸载已构造的插件实例；它不发现文件、不加载二进制，也不监听目录变化。
- 新插件应作为本仓库内源码变更开发、测试和注册；若要提供外部扩展，需先设计公开 SDK 或稳定 HTTP/gRPC 扩展协议。

## 基础接口

```go
type Plugin interface {
    Name() string
    Version() string
    Description() string
    Author() string
    Type() PluginType
    Initialize(config *model.PluginConfig) error
    Start(ctx context.Context) error
    Stop() error
    Health() HealthStatus
}

type HookFunc func(pluginName string, data *model.HookData) error
```

插件种类：`EnginePlugin`、`ProcessorPlugin`、`ExporterPlugin`、`NotifierPlugin`。`EnginePlugin` 额外定义 `Translate`、`Search`、`Normalize`、`SupportedFields`、`MaxPageSize` 和 `RateLimit`。

`model.PluginConfig` 提供 `Enabled`、`APIKey`、`BaseURL`、`QPS`、`Timeout` 与受控的 `Extra` 扩展字段。不要在文档或日志中输出 `APIKey`。

## 生命周期

```text
构造 Plugin 实例
  → PluginManager.LoadPlugin(plugin, *PluginConfig)
  → Initialize → Register
  → StartPlugin / StartAll
  → StopPlugin / StopAll
  → UnloadPlugin（停止后注销）
```

`LoadPlugin` 会触发 before/after load hook；启动、停止、卸载和健康失败也有对应 hook。Hook 参数是 `*model.HookData`，不再是 `map[string]interface{}`。

## 开发与验证

1. 在仓库内新增实现，避免用外部模块导入 `internal/*`。
2. 以 `internal/plugin/example_engine_plugin.go` 为最小可运行参考。
3. 为初始化、启动失败、健康状态、注册冲突和 hook 添加测试。
4. 运行：

```bash
go test -race ./internal/plugin/...
go test -race ./...
```

详细开发步骤见 [PLUGIN_DEVELOPMENT_GUIDE.md](PLUGIN_DEVELOPMENT_GUIDE.md)。
