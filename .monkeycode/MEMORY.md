# 用户指令记忆

本文件记录了用户的指令、偏好和教导，用于在未来的交互中提供参考。

## 格式

### 用户指令条目
用户指令条目应遵循以下格式：

[用户指令摘要]
- Date: [YYYY-MM-DD]
- Context: [提及的场景或时间]
- Instructions:
  - [用户教导或指示的内容，逐行描述]

### 项目知识条目
Agent 在任务执行过程中发现的条目应遵循以下格式：

[项目知识摘要]
- Date: [YYYY-MM-DD]
- Context: Agent 在执行 [具体任务描述] 时发现
- Category: [代码结构|代码模式|代码生成|构建方法|测试方法|依赖关系|环境配置]
- Instructions:
  - [具体的知识点，逐行描述]

## 去重策略
- 添加新条目前，检查是否存在相似或相同的指令
- 若发现重复，跳过新条目或与已有条目合并
- 合并时，更新上下文或日期信息
- 这有助于避免冗余条目，保持记忆文件整洁

## 条目

### CORS 中间件对 Bridge 路由的绕过机制
- Date: 2026-05-07
- Context: Agent 在修复浏览器扩展 CORS 错误时发现
- Category: 代码模式
- Instructions:
  - `corsMiddleware` 会跳过 `/api/screenshot/bridge/*` 路径的 CORS origin 白名单检查
  - Bridge 路由有自己的认证机制（loopback IP 校验 + bearer token）
  - 对 bridge 路径直接返回 `Access-Control-Allow-Origin: *`，允许任意 origin（包括 `chrome-extension://`）
  - 辅助函数 `isScreenshotBridgePath()` 定义在 `web/middleware_auth.go` 中，用于判断路径是否属于 bridge API
  - `isOriginAllowed()` 函数也额外允许了 `chrome-extension://` 前缀的 origin，但此逻辑不应用于 bridge 路由（bridge 路由完全绕过 CORS 检查）

### 空间搜索引擎默认启用机制
- Date: 2026-05-07
- Context: Agent 在修复"未加载空间引擎导致查询失败"问题时发现
- Category: 代码结构
- Instructions:
  - 在 `internal/config/config.go` 的 `applyDefaults` 函数中设置所有搜索引擎 `Enabled = true`
  - FOFA 同时设置 `UseWebAPI = true` 作为默认值，允许在没有 API Key 时仍能使用 Web 模式
  - 验证函数保持完整性，仅在 `UseWebAPI = false` 时才要求提供 API Key
  - 所有引擎：Quake、ZoomEye、Hunter、FOFA、Shodan
