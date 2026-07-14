# UniMap 插件开发指南

> 当前插件只能作为 UniMap 仓库内的源码扩展开发。`internal/plugin` 不能被仓库外模块导入，也没有动态二进制发现或热加载功能。

## 前置条件

- Go 1.26.5。
- 在本仓库内创建实现和测试。
- 阅读 [PLUGIN_ARCHITECTURE.md](PLUGIN_ARCHITECTURE.md) 与 `internal/plugin/example_engine_plugin.go`。

## 最小 EnginePlugin

```go
type MyEnginePlugin struct {
    initialized bool
}

func (p *MyEnginePlugin) Name() string        { return "my-engine" }
func (p *MyEnginePlugin) Version() string     { return "0.1.0" }
func (p *MyEnginePlugin) Description() string { return "example" }
func (p *MyEnginePlugin) Author() string      { return "team" }
func (p *MyEnginePlugin) Type() plugin.PluginType { return plugin.PluginTypeEngine }

func (p *MyEnginePlugin) Initialize(cfg *model.PluginConfig) error {
    if cfg == nil || !cfg.Enabled {
        return fmt.Errorf("plugin is disabled")
    }
    p.initialized = true
    return nil
}

func (p *MyEnginePlugin) Start(ctx context.Context) error {
    if !p.initialized { return fmt.Errorf("plugin not initialized") }
    return nil
}

func (p *MyEnginePlugin) Stop() error { return nil }
func (p *MyEnginePlugin) Health() plugin.HealthStatus {
    return plugin.HealthStatus{Healthy: p.initialized, Message: "ok"}
}
```

再实现 `EnginePlugin` 要求的 `Translate`、`Search`、`Normalize`、`SupportedFields`、`MaxPageSize` 和 `RateLimit`。方法签名以 `internal/plugin/plugin.go` 为准；不要从旧示例复制 `Initialize(map[string]interface{})`。

## 注册

在组成根（现有服务初始化代码）中构造并加载实例：

```go
manager := plugin.NewPluginManager()
err := manager.LoadPlugin(myPlugin, &model.PluginConfig{
    Enabled: true,
    BaseURL: "https://api.example.invalid",
})
if err != nil { return err }
if err := manager.StartPlugin(myPlugin.Name()); err != nil { return err }
```

不要把 API Key 写入源码、测试 fixture 或文档。通过本地受控配置或测试环境变量注入。

## Hook

```go
manager.GetHooks().RegisterHook(plugin.HookAfterQuery,
    func(pluginName string, data *model.HookData) error {
        // data.Query、data.Engines、data.Result 等是类型化入口
        return nil
    },
)
```

Hook 不应阻塞请求路径或泄露结果中的凭证、Cookie 与内部错误。

## 测试

覆盖至少以下场景：

- `Initialize` 接收有效/无效 `*model.PluginConfig`。
- 未初始化时 `Start` 失败。
- 同名插件注册冲突。
- Hook 成功与失败传播。
- `Stop`、`UnloadPlugin` 和健康状态转换。

```bash
go test -race ./internal/plugin/...
go test -race ./...
```
