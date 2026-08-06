# ADR 0006：统一无图形浏览器运行时

- 状态：Accepted
- 日期：2026-07-19

## 背景

截图、浏览器查询、调度和巡检曾通过不同路径选择 CDP 或 Extension。Extension 服务对象已创建但没有在线客户端时，部分调用仍会把它当作可用；本地 Chrome 路径检测也可能忽略显式配置。云主机通常没有图形会话，批量任务若无限制启动 Chromium 还会造成 OOM。

## 决策

1. 所有截图模式都初始化 ScreenshotRouter，业务调用不再根据 Bridge 对象存在与否绕过路由。
2. 路由分别记录配置模式和健康降级后的活动模式；readiness 按配置模式及 fallback 语义判断。
3. CDP 使用 Chrome/Chromium 新版 headless。显式 `screenshot.chrome_path` 或 `UNIMAP_CHROME_PATH` 必须真实存在，否则不静默切换到其他浏览器。Windows readiness 静态验证 PE，不执行可能激活用户浏览器会话的 `chrome.exe --version`；Router 没有 CDP provider 时不执行 CDP 探针。
4. Manager 对全部 allocator 使用共享会话槽；默认上限为 2，云容器基线为 1。指定持久化 user-data-dir 时自动串行，避免 Chromium profile 锁冲突。
5. 容器内置 Chromium、CJK 字体、持久化数据/截图/profile 目录和 256 MiB `/dev/shm`，健康检查使用 `/health/ready`。仅容器基线显式关闭 Chrome sandbox，普通主机保留 sandbox。
6. 默认测试层不启动真实浏览器。CI 在独立的 `headless_e2e` 测试层和空 `DISPLAY` 下运行真实 Chrome，验证 JavaScript DOM 采集、PNG 截图和篡改检测渲染，不以“能够编译”代替浏览器验收。

## 结果与边界

Linux/CDP 不再依赖桌面、Xvfb 或 VNC；Extension 仍需要用户浏览器在线，这是其设计边界，不作为纯云主机的默认模式。会话限制优先保证稳定性，可能降低吞吐；可在没有共享 profile 且资源充足时提高 `screenshot.max_sessions`。真实云供应商的镜像、网络、内存和安全组仍需部署前做一次环境验收。
