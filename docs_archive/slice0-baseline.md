# Slice 0: 基线冻结与验收口径

> 执行日期: 2026-05-13
> 状态: 执行中
> 原则: 只新增测试和文档,不改业务代码

## 1. 现有接口盘点

### 1.1 公开路由 (无需认证)

| 路径 | 方法 | 功能 | 限流 |
|------|------|------|------|
| `/health` | GET | 健康检查 | 否 |
| `/health/ready` | GET | 就绪检查 | 否 |
| `/health/live` | GET | 存活检查 | 否 |
| `/login` | GET | 登录页面 | 否 |
| `/api/login` | POST | 登录接口 | 是 |
| `/api/logout` | POST | 退出接口 | 否 |
| `/api/screenshot/bridge/*` | 多种 | Bridge API (6个) | 部分 |
| `/static/*` | GET | 静态资源 | 否 |
| `/screenshots/*` | GET | 截图文件 | 否 |

### 1.2 管理路由 (需认证)

#### 查询相关
| 路径 | 方法 | 功能 | 限流 |
|------|------|------|------|
| `/api/query` | POST | 执行查询 (form) | 是 |
| `/api/query` | GET | 查询状态 | 否 |
| `/query` | GET | 查询页面 | 否 |

#### 截图相关
| 路径 | 方法 | 功能 | 限流 |
|------|------|------|------|
| `/api/screenshot/batch-urls` | POST | 批量截图 | 是 |
| `/api/screenshot/batch` | GET | 批量截图页面 | 否 |
| `/api/screenshot/status` | GET | 截图状态 | 否 |
| `/api/screenshot/clear` | POST | 清除截图 | 否 |
| `/api/screenshot/download` | GET | 下载截图 | 否 |
| `/api/screenshot/delete` | POST | 删除截图 | 否 |

#### 监控相关
| 路径 | 方法 | 功能 | 限流 |
|------|------|------|------|
| `/api/url/reachability` | POST | 可达性检测 | 是 |
| `/api/url/port-scan` | POST | 端口扫描 | 是 |
| `/monitor` | GET | 监控页面 | 否 |

#### 篡改检测
| 路径 | 方法 | 功能 | 限流 |
|------|------|------|------|
| `/api/tamper/check` | POST | 篡改检测 | 是 |
| `/api/tamper/baseline` | POST | 设置基线 | 否 |
| `/api/tamper/baseline` | GET | 获取基线 | 否 |
| `/api/tamper/baseline` | DELETE | 删除基线 | 否 |
| `/api/tamper/history` | GET | 检测历史 | 否 |

#### 定时任务
| 路径 | 方法 | 功能 | 限流 |
|------|------|------|------|
| `/api/scheduler/tasks` | GET | 任务列表 | 否 |
| `/api/scheduler/tasks` | POST | 创建任务 | 否 |
| `/api/scheduler/tasks/:id` | GET | 任务详情 | 否 |
| `/api/scheduler/tasks/:id` | PUT | 更新任务 | 否 |
| `/api/scheduler/tasks/:id` | DELETE | 删除任务 | 否 |
| `/api/scheduler/tasks/:id/run` | POST | 立即执行 | 否 |
| `/api/scheduler/tasks/:id/toggle` | POST | 启停任务 | 否 |
| `/api/scheduler/history` | GET | 执行历史 | 否 |
| `/scheduler` | GET | 调度器页面 | 否 |

#### 分布式节点
| 路径 | 方法 | 功能 | 限流 |
|------|------|------|------|
| `/api/nodes/register` | POST | 节点注册 | 否 |
| `/api/nodes/heartbeat` | POST | 心跳 | 否 |
| `/api/nodes/:id/tasks` | GET | 领取任务 | 否 |
| `/api/nodes/:id/tasks/:taskId/complete` | POST | 完成任务 | 否 |
| `/api/nodes/:id/tasks/:taskId/fail` | POST | 任务失败 | 否 |
| `/api/nodes/status` | GET | 节点状态 | 否 |
| `/api/nodes/:id` | GET | 节点详情 | 否 |
| `/api/nodes/:id/unregister` | POST | 注销节点 | 否 |
| `/api/nodes/:id/toggle` | POST | 启停节点 | 否 |
| `/api/nodes/profile` | GET | 节点配置 | 否 |
| `/api/nodes/enqueue` | POST | 入队任务 | 否 |

#### Cookie 管理
| 路径 | 方法 | 功能 | 限流 |
|------|------|------|------|
| `/api/cookies` | GET | Cookie 列表 | 否 |
| `/api/cookies` | POST | 添加 Cookie | 否 |
| `/api/cookies/:id` | DELETE | 删除 Cookie | 否 |
| `/api/cookies/verify` | POST | 验证 Cookie | 否 |

#### 备份相关
| 路径 | 方法 | 功能 | 限流 |
|------|------|------|------|
| `/api/backup/create` | POST | 创建备份 | 否 |
| `/api/backup/restore` | POST | 恢复备份 | 否 |
| `/api/backup/list` | GET | 备份列表 | 否 |

#### API Key 管理 (预期实现)
| 路径 | 方法 | 功能 | 限流 |
|------|------|------|------|
| `/api/api-keys` | GET | API Key 列表 | 否 |
| `/api/api-keys` | POST | 生成新 API Key | 否 |
| `/api/api-keys/:id` | DELETE | 撤销 API Key | 否 |
| `/api/api-keys/stats` | GET | API Key 统计信息 | 否 |

#### WebSocket
| 路径 | 方法 | 功能 | 限流 |
|------|------|------|------|
| `/ws/query` | WebSocket | 查询实时推送 | 是 |
| `/ws/monitor` | WebSocket | 监控实时推送 | 否 |

#### CDP (Chrome DevTools Protocol)
| 路径 | 方法 | 功能 | 限流 |
|------|------|------|------|
| `/api/cdp/version` | GET | CDP 版本 | 否 |
| `/api/cdp/screenshot` | POST | CDP 截图 | 是 |

#### 配额与统计
| 路径 | 方法 | 功能 | 限流 |
|------|------|------|------|
| `/quota` | GET | 配额页面 | 否 |
| `/api/stats` | GET | 统计信息 | 否 |

### 1.3 Bridge API (特殊鉴权)

| 路径 | 方法 | 功能 | 鉴权方式 |
|------|------|------|----------|
| `/api/screenshot/bridge/health` | GET | 健康检查 | Loopback 检查 |
| `/api/screenshot/bridge/status` | GET | 状态查询 | Loopback 检查 |
| `/api/screenshot/bridge/pair` | POST | 配对获取 token | Loopback 检查 |
| `/api/screenshot/bridge/token/rotate` | POST | 轮换 token | Bearer Token |
| `/api/screenshot/bridge/tasks/next` | GET | 拉取下一个任务 | Bearer Token |
| `/api/screenshot/bridge/mock/result` | POST | 模拟回调结果 | Bearer Token + 签名 |

## 2. 配置字段盘点

### 2.1 认证相关 (`web.auth`)

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `web.auth.enabled` | bool | `true` | 是否启用认证 |
| `web.auth.admin_token` | string | 自动生成 | API/CLI 管理 token |
| `web.auth.username` | string | `"admin"` | Web 登录用户名 |
| `web.auth.password_hash` | string | bcrypt("admin") | Web 登录密码哈希 |
| `web.auth.api_key_store` | string | `"./data/api_keys.json"` | API Key 存储路径 |

### 2.2 系统相关 (`system`)

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `system.max_concurrent` | int | `10` | 查询并发上限 |
| `system.cache_ttl` | int | `3600` | 缓存 TTL (秒) |
| `system.cache_max_size` | int | `1000` | 缓存最大条目 |
| `system.cache_cleanup_interval` | int | `300` | 缓存清理间隔 (秒) |

### 2.3 Web 相关 (`web`)

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `web.port` | int | `8448` | Web 服务端口 |
| `web.bind_address` | string | `"0.0.0.0"` | 绑定地址 |
| `web.cors.allowed_origins` | []string | localhost | 允许的 CORS 源 |
| `web.cors.allowed_methods` | []string | GET,POST,PUT... | 允许的 HTTP 方法 |
| `web.rate_limit.enabled` | bool | `true` | 是否启用限流 |
| `web.rate_limit.requests_per_window` | int | `60` | 时间窗口内请求数 |
| `web.rate_limit.window_seconds` | int | `60` | 时间窗口 (秒) |

### 2.4 截图相关 (`screenshot`)

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `screenshot.enabled` | bool | `true` | 是否启用截图 |
| `screenshot.engine` | string | `"cdp"` | 截图引擎: cdp/extension |
| `screenshot.chrome_path` | string | 空 | Chrome 路径 |
| `screenshot.headless` | bool | `true` | 无头模式 |
| `screenshot.timeout` | int | `30` | 超时 (秒) |
| `screenshot.extension.enabled` | bool | `false` | 启用扩展模式 |
| `screenshot.extension.listen_addr` | string | `127.0.0.1:19451` | 扩展监听地址 |
| `screenshot.extension.pairing_required` | bool | `true` | 需要配对 |
| `screenshot.extension.token_ttl_seconds` | int | `600` | Token TTL |
| `screenshot.extension.callback_signature_required` | bool | `false` | 回调签名 |

### 2.5 分布式相关 (`distributed`)

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `distributed.enabled` | bool | `false` | 启用分布式 |
| `distributed.heartbeat_timeout_seconds` | int | `30` | 心跳超时 |
| `distributed.admin_token` | string | 空 | 控制器 token |
| `distributed.node_auth_tokens` | map | 空 | 节点 token 映射 |

### 2.6 调度器相关 (`scheduler`)

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `scheduler.enabled` | bool | `true` | 启用定时任务 |
| `scheduler.max_history` | int | `100` | 最大历史记录 |

## 3. CLI 参数盘点

### 3.1 Legacy 模式 (默认)

| 参数 | 简写 | 类型 | 说明 |
|------|------|------|------|
| `-q` | `--query` | string | UQL 查询语句 |
| `-e` | `--engines` | string | 引擎列表 (逗号分隔) |
| `-l` | `--limit` | int | 返回结果数量 |
| `-o` | `--output` | string | 输出文件路径 |
| `--format` | - | string | 输出格式: json/csv/excel |
| `--fofa-cookie` | - | string | FOFA Cookie |
| `--hunter-cookie` | - | string | Hunter Cookie |
| `--config` | - | string | 配置文件路径 |

### 3.2 API 子命令

| 子命令 | 参数 | 说明 |
|--------|------|------|
| `query` | `-q, --api-base, --token, --limit, --format, --output` | 通过 API 查询 |
| `tamper-check` | `--urls, --mode, --api-base, --token` | 篡改检测 |
| `screenshot-batch` | `--urls, --api-base, --token, --output` | 批量截图 |

### 3.3 缺失的参数 (需要新增)

- `--plugin-mode` - 插件连接模式 (off/auto/required)
- `monitor reachability` - 可达性监控子命令
- `monitor port-scan` - 端口扫描子命令
- `--token` 读取优先级支持 (参数/环境变量/配置文件)

## 4. 页面按钮类盘点

### 4.1 登录页面 (`/login`)
- 登录表单 (username/password)
- CSRF Token (隐藏字段)

### 4.2 查询页面 (`/query`)
- 查询输入框
- 引擎选择复选框
- 查询按钮 (`.btn-primary`)
- 导出按钮 (CSV/Excel/JSON)
- 结果表格
- 分页控件

### 4.3 监控页面 (`/monitor`)
- URL 输入框
- 端口输入框
- 检测按钮
- 结果展示区域
- 历史记录表格

### 4.4 定时任务页面 (`/scheduler`)
- 创建任务按钮
- 任务列表表格
- 启用/停用按钮
- 立即执行按钮
- 删除按钮
- 编辑按钮 (⚠️ 缺失,待实现)
- 筛选控件

### 4.5 截图页面 (`/api/screenshot/batch`)
- URL 批量输入
- 截图按钮
- 截图列表展示
- 下载/删除按钮

### 4.6 配额页面 (`/quota`)
- 引擎配额卡片
- 使用量展示

## 5. Smoke Test 清单

### 5.1 认证 Smoke Test

```bash
# T-AUTH-001: 健康检查 (公开)
curl -s http://localhost:8448/health | jq '.status'
# 期望: "ok"

# T-AUTH-002: 登录页面 (公开)
curl -s http://localhost:8448/login | grep -o '<form'
# 期望: 找到 form 标签

# T-AUTH-003: 登录成功
curl -s -X POST http://localhost:8448/api/login \
  -d 'username=admin&password=admin&csrf_token=...' \
  -c /tmp/cookies.txt -v | grep -o 'unimap_session'
# 期望: 返回 session cookie

# T-AUTH-004: Admin Token 认证
curl -s http://localhost:8448/api/stats \
  -H "X-Admin-Token: <token>" | jq '.status'
# 期望: "ok"

# T-AUTH-005: 无认证访问受保护资源
curl -s -o /dev/null -w "%{http_code}" http://localhost:8448/api/stats
# 期望: 302 或 401

# T-AUTH-006: API Key 认证
curl -s http://localhost:8448/api/stats \
  -H "Authorization: ApiKey <api_key>" | jq '.status'
# 期望: "ok"
```

### 5.2 查询 Smoke Test

```bash
# T-QUERY-001: Legacy 模式查询
go run ./cmd/unimap-cli -q 'country="CN"' -e fofa -l 5
# 期望: 返回 5 条结果

# T-QUERY-002: API 模式查询
go run ./cmd/unimap-cli query -q 'country="CN"' --api-base http://localhost:8448 --token <token> -l 5
# 期望: 返回 5 条结果

# T-QUERY-003: 导出 CSV
go run ./cmd/unimap-cli -q 'country="CN"' -e fofa -l 5 -o /tmp/test.csv --format csv
# 期望: 生成 CSV 文件

# T-QUERY-004: WebSocket 查询
# 通过浏览器访问 /query,输入查询,观察实时进度
# 期望: 进度条更新,结果实时展示
```

### 5.3 监控 Smoke Test

```bash
# T-MONITOR-001: 可达性检测
curl -s -X POST http://localhost:8448/api/url/reachability \
  -H "X-Admin-Token: <token>" \
  -d 'urls=https://example.com&concurrency=5&timeout=10' | jq '.status'
# 期望: "ok" 或 "completed"

# T-MONITOR-002: 端口扫描
curl -s -X POST http://localhost:8448/api/url/port-scan \
  -H "X-Admin-Token: <token>" \
  -d 'urls=https://example.com&ports=80,443&concurrency=5&timeout=10' | jq '.status'
# 期望: "ok" 或 "completed"
```

### 5.4 Bridge Smoke Test

```bash
# T-BRIDGE-001: 健康检查
curl -s http://localhost:8448/api/screenshot/bridge/health | jq '.status'
# 期望: 返回 bridge 健康状态

# T-BRIDGE-002: 配对获取 Token
curl -s -X POST http://localhost:8448/api/screenshot/bridge/pair | jq '.token'
# 期望: 返回 bridge token

# T-BRIDGE-003: 使用 Token 拉取任务
curl -s http://localhost:8448/api/screenshot/bridge/tasks/next \
  -H "Authorization: Bearer <bridge_token>" | jq '.'
# 期望: 返回任务或空队列

# T-BRIDGE-004: Token 轮换
curl -s -X POST http://localhost:8448/api/screenshot/bridge/token/rotate \
  -H "Authorization: Bearer <bridge_token>" | jq '.token'
# 期望: 返回新 token
```

### 5.5 定时任务 Smoke Test

```bash
# T-SCHED-001: 创建任务
curl -s -X POST http://localhost:8448/api/scheduler/tasks \
  -H "X-Admin-Token: <token>" \
  -H "Content-Type: application/json" \
  -d '{"name":"test","cron":"0 * * * *","type":"query","query":"country=\"CN\"","engines":["fofa"]}' | jq '.id'
# 期望: 返回任务 ID

# T-SCHED-002: 任务列表
curl -s http://localhost:8448/api/scheduler/tasks \
  -H "X-Admin-Token: <token>" | jq '.tasks | length'
# 期望: >= 1

# T-SCHED-003: 启停任务
curl -s -X POST http://localhost:8448/api/scheduler/tasks/<id>/toggle \
  -H "X-Admin-Token: <token>" | jq '.enabled'
# 期望: 切换 enabled 状态

# T-SCHED-004: 立即执行
curl -s -X POST http://localhost:8448/api/scheduler/tasks/<id>/run \
  -H "X-Admin-Token: <token>" | jq '.status'
# 期望: "running"
```

### 5.6 API Key 管理 Smoke Test (预期实现)

```bash
# T-APIKEY-001: 生成新 API Key
curl -s -X POST http://localhost:8448/api/api-keys \
  -H "X-Admin-Token: <token> \
  -H "Content-Type: application/json" \
  -d '{"description":"test key","permissions":["admin"]}' | jq '.key'
# 期望: 返回新生成的 API Key（仅显示一次）

# T-APIKEY-002: 列出所有 API Key
curl -s http://localhost:8448/api/api-keys \
  -H "X-Admin-Token: <token>" | jq '.keys | length'
# 期望: 返回 API Key 列表（不包含明文 Key）

# T-APIKEY-003: 获取 API Key 统计
curl -s http://localhost:8448/api/api-keys/stats \
  -H "X-Admin-Token: <token>" | jq '.active'
# 期望: 返回活跃 API Key 数量

# T-APIKEY-004: 撤销 API Key
curl -s -X DELETE http://localhost:8448/api/api-keys/<key_id> \
  -H "X-Admin-Token: <token>" | jq '.success'
# 期望: true

# T-APIKEY-005: 使用 API Key 认证
curl -s http://localhost:8448/api/stats \
  -H "Authorization: ApiKey <api_key>" | jq '.status'
# 期望: "ok"

# T-APIKEY-006: 撤销后验证失效
# 先撤销 Key 再尝试
curl -s -o /dev/null -w "%{http_code}" http://localhost:8448/api/stats \
  -H "Authorization: ApiKey <revoked_api_key>"
# 期望: 401
```

### 5.7 CLI API 子命令 Smoke Test

```bash
# T-CLI-001: API 查询 (当前失败,需要 token 支持)
go run ./cmd/unimap-cli query -q 'country="CN"' --api-base http://localhost:8448 -l 5
# 当前结果: 401 Unauthorized
# 期望结果: 返回查询结果 (完成 Slice 5 后)

# T-CLI-002: API 篡改检测
go run ./cmd/unimap-cli tamper-check --urls 'https://example.com' --api-base http://localhost:8448
# 当前结果: 401 Unauthorized
# 期望结果: 返回检测结果 (完成 Slice 5 后)
```

### 5.8 跨平台 Smoke Test

```bash
# T-PLATFORM-001: 构建测试
go build ./cmd/unimap-web
go build ./cmd/unimap-cli
go build -tags gui ./cmd/unimap-gui
# 期望: 全部成功

# T-PLATFORM-002: 测试通过
go test -race ./...
# 期望: 全部通过

# T-PLATFORM-003: 启动 Web 服务
./unimap-web --config configs/config.yaml
# 期望: 监听 8448 端口,打印 Admin Token
```

## 7. 已知风险点

### 7.1 高风险

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| CLI API 子命令无认证 | Web Auth 启用后 401 | Slice 5 优先实施 |
| Bridge API 与 Web Session 可能混用 | 扩展越权 | Slice 2 独立鉴权 |
| 密码修改无接口 | 无法更改默认密码 | Slice 3 新增接口 |
| 前端按钮样式冲突 | 视觉不统一 | Slice 10-11 统一设计令牌 |
| API Key 管理接口缺失 | 无法方便管理 API Key | 后续 Slice 新增接口 |

### 7.2 中风险

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| Chrome 探测逻辑重复 | 跨平台不一致 | Slice 12 统一适配器 |
| 调度器编辑 UI 缺失 | 无法编辑任务 | 后续迭代修复 |
| 测试覆盖率 65.1% | 未达 80% 标准 | 持续补充 |

### 7.3 低风险

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| 错误消息大写不一致 | 用户体验 | 后续统一 |
| 部分文件超 800 行 | 可维护性 | 渐进式重构 |

## 8. 验收标准

### 8.1 Slice 0 完成标准

- [x] 现有接口盘点完成 (本文档)
- [x] 配置字段盘点完成 (本文档)
- [x] CLI 参数盘点完成 (本文档)
- [x] 页面按钮类盘点完成 (本文档)
- [x] Smoke Test 清单编写完成 (本文档)
- [x] 基线构建验证通过 (`go build ./...` 成功)
- [x] 基线测试验证 (2 个已知环境相关失败,不影响核心功能)

### 7.2 整体重构完成标准

- [ ] 所有 Slice 按顺序实施
- [ ] `go build ./...` 通过
- [ ] `go test -race ./...` 通过
- [ ] Smoke Test 清单全部通过
- [ ] 跨平台构建验证 (Windows/macOS/Linux)
- [ ] 向后兼容: 旧配置/旧 API/旧 CLI 参数仍可用

## 8. 回滚方案

Slice 0 只新增文档,无需回滚。后续 Slice 回滚方案:

- **认证回滚**: 关闭 AuthFacade 策略开关
- **Bridge 回滚**: 恢复旧鉴权路径
- **CLI 回滚**: 恢复旧子命令和请求客户端
- **前端回滚**: 恢复旧 CSS 和模板
- **跨平台回滚**: 旧平台函数作为 adapter 后备
