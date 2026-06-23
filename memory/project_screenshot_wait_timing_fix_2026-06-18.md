---
name: screenshot_wait_timing_fix_2026-06-18
description: 截图等待时间统一15秒 + 飞书应用图片推送修复
metadata:
  type: project
---

# 截图等待时间 + 飞书应用推送修复（2026-06-18）

## 问题

### P1. 截图等待时间不足
- **collect_and_capture** action 没有额外等待时间（只有 `collect`/`screenshot` 有 4 秒）
- SPA 引擎（FOFA/Hunter/ZoomEye/Quake）搜索结果异步渲染，截图时页面未完全加载
- 特别是 Hunter 登录态校验较慢，截图显示未登录页面

### P2. 飞书应用通知图片上传失败
- `extractImagePaths()` 只能识别 `→`（批量截图格式），不能识别 `保存:`（搜索引擎截图格式）
- 飞书应用通知中的截图图片上传失败

### P3. 搜索引擎截图任务 payload 格式
- `engine` 字段必须在 `extra` 对象中传递（不是一级字段）
- 正确格式：`{"query": "...", "extra": {"engine": "zoomeye"}}`

## 修复

### 1. background.js — 统一15秒固定等待
- 文件：`tools/extension-screenshot/src/background.js`
- 条件：`collect` / `screenshot` / `collect_and_capture` 三种 action
- 第1段：**15秒** 固定等待（覆盖所有引擎的页面加载和登录态校验）
- 滚动到底部触发懒加载内容，再滚回顶部
- 第2段：**2秒** 稳定等待（滚动后的内容渲染）

### 2. scheduler_notify.go — 添加`保存:`格式识别
- 文件：`internal/scheduler/scheduler_notify.go`
- 新增 Pattern 2 识别 `保存:` （搜索引擎截图格式）
- 原有的 `→` Pattern 保留（批量截图格式）
- `go build ./...` / `go test ./...` 全部通过

## 验证

| 引擎 | 耗时 | 截图 | 飞书推送 | 登录态 |
|------|------|------|---------|-------|
| ZoomEye | 17.7秒 | ✅ 完整 | ✅ 带图片 | ✅ |
| FOFA | 20.6秒 | ✅ 完整 | ✅ 带图片 | ✅ |
| Hunter | 26.9秒 | ✅ 完整 | ✅ 带图片 | ✅ 登录态正常 |
| Quake | 17.3秒 | ✅ 完整 | ✅ 带图片 | ✅ |
| Shodan | 18.7秒 | ✅ 完整 | ✅ 带图片 | ✅ |

## 关键技术知识

- **collect_and_capture 等待**：必须同时覆盖 `collect`、`screenshot`、`collect_and_capture` 三种 action
- **SPA 页面统一等待**：不同引擎页面加载速度不同，统一使用最长等待（15秒）比差异化更简单可靠
- **路径提取双格式**：`scheduler_notify.go:extractImagePaths` 同时支持 `→` 和 `保存:` 两种路径分隔符
- **search_screenshot payload**：`engine` 必须在 `extra` 对象中

## 相关文件

- `tools/extension-screenshot/src/background.js` — 等待时间逻辑
- `internal/scheduler/scheduler_notify.go` — 图片路径提取逻辑

**为什么：** SPA 引擎搜索结果页异步渲染，固定时间等待+滚动触发懒加载比差异化策略更可靠
**如何应用：** 修改背景页后必须刷新 Chrome 扩展（chrome://extensions/）