# Chrome 手动测速工具

该工具仅用于人工验证 Chrome 启动、无头导航和网络耗时。它带有
`manual_browser_test` 构建标签，因此不会进入常规 `go test ./...`、
`go build ./...` 或应用启动流程。

```powershell
go run -tags manual_browser_test ./tools/chrome-speedtest
go run -tags manual_browser_test ./tools/chrome-speedtest -timeout 45s https://example.com
```

工具显式使用无头模式，并在同一个 Chrome 进程中依次访问所有 URL。
