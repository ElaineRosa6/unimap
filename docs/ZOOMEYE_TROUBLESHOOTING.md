# ZoomEye API 故障排查

> 最后外部核对：2026-07-13。套餐、额度和 API 权限以 [ZoomEye 官方帮助](https://www.zoomeye.ai/help) 与账户页面为准；不要依赖旧的“注册用户必然有免费 API 权限”说法。

## 遇到 402、403 或额度不足

1. 在当前 ZoomEye 账户页面确认套餐是否包含 API 权限、当前 API 速率和点数/额度。
2. 确认 API Key 未撤销、未过期，且 UniMap 中使用的是正确的 `engines.zoomeye.api_key`。
3. 用一个最小、已授权的查询验证账户，避免在排障过程中消耗大量额度。
4. 记录 HTTP 状态、平台请求 ID（若有）和脱敏错误信息；不要记录完整 API Key。
5. 若平台侧显示权限正常但仍失败，向 ZoomEye 官方支持提交脱敏证据。

## UniMap 侧检查

- 读取 `configs/config.yaml` 中的 `engines.zoomeye` 配置，或使用 `${ZOOMEYE_API_KEY}` 这类显式环境变量占位符。
- 检查服务日志中的脱敏响应错误和网络超时。
- 通过 Web 或 CLI 发起小范围查询；Web API 使用 `/api/v1/query` 表单协议。
- 不存在 `cmd/debug-zoomeye/main.go` 调试程序，不要执行历史文档中的该命令。

## 不做的假设

- 不假设手机验证会自动授予 API 权限。
- 不假设免费账户拥有固定月度 API 配额。
- 不在本文固定 V2/V3、端点或 header 细节；外部 API 契约变更时应先查官方开发者文档，并同步更新适配器测试。
