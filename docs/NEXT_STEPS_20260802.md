# UniMap V1 后续实施清单

> 核对日期：2026-08-02（Asia/Shanghai）；当前事实以代码、测试和 [七引擎浏览器验收记录](BROWSER_SEVEN_ENGINE_VERIFICATION_2026-08-02.md) 为准。
>
> **执行队列已移交：** 2026-08-20 起后续 agent 按 [推进计划书](AGENT_CONTINUATION_PLAN_2026-08-20.md) 推进。本文保留为 08-02 收口快照，不再当待办顺序。

## 本轮已完成

1. API 与 Browser 并行成功语义闭合：API 失败而 Browser 非空时返回 HTTP 200 + `status=partial`；两路失败才整体报错。
2. HTTP 与调度浏览器工作流统一为一次 SQLite 合并持久化，避免重复历史。
3. 七引擎 Web UI、设置、Cookie 导入/状态、调度默认登录检查接通；Censys/DayDayMap 无 API 凭据时注册 Web-only adapter。
4. 七引擎真实 Bridge 结构化采集均非空；Shodan 当前登录态已通过。
5. Censys/DayDayMap 的查询 URL、Bridge task `query`、L1/L3 与凭据交接修正完成；两者 Bridge 实测通过。
6. Bridge 到 CDP 支持 Cookie + 同源 Web Storage；attached CDP 也应用显式 Cookie。
7. Censys/DayDayMap 截图、调度、SQLite 单历史闭环通过；通知加入超时与重试。
8. 当前代码已以 CGO/SQLite 静态 musl 二进制发布到云端派生镜像；健康、配置持久化、容器重启、旧镜像回滚和再次发布均通过。fixture 静态二进制已上传 staging。
9. 云端飞书应用通知已连续两次真实发送成功；通知配置回滚、重新应用、重启健康和临时明文载荷清理均完成。

## 仍需完成或受外部条件限制

| 项目 | 当前结论 | 下一动作 |
|---|---|---|
| FOFA/ZoomEye/Shodan CDP 定级 | Bridge 已通过；CDP 非空证据未形成 | 在可用 CDP 出口逐引擎记录通过/受限 |
| Censys CDP | 交接 1 Cookie + 16 Web Storage；挑战结构化识别，自动回退 Bridge 9 条 | 页面策略允许时复验原生 CDP，不采用脆弱绕过 |
| DayDayMap CDP | 交接 15 Web Storage；受控 loopback SOCKS5 下取得 10 条与 PNG | 已完成 |
| 云端 DNS rebinding + 真实变化 | fixture 已 staging；缺少域名/DNS 控制输入 | 提供受控域名、DNS 编辑凭据、控制/目标 URL 后启动 |
| 飞书闭环重跑 | 云端连续两次返回 HTTP 200；配置回滚和重新应用通过 | 已完成，保留回滚脚本与脱敏验证记录 |
| 发布 | 本地全量门禁与云端热更新/回滚已通过 | 保留派生镜像与回滚脚本，后续按正式版本打标签 |

## 最终门禁

```powershell
node --test tools/extension-screenshot/test/*.test.mjs
go test -race ./...
go vet ./...
go build ./...
git diff --check
```

## 需要提供的外部输入

云端安全 fixture 的完整验收需要：受控测试域名/子域、可编辑该域 DNS 的短期凭据（或等价接口）、fixture 控制 URL 与变化目标 URL。凭据只放入受保护环境/secret channel，不写入仓库或命令历史。
