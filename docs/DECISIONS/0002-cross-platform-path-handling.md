# 0002 跨平台路径处理与数据目录标准化

## 状态

已接受

## 背景

审计发现项目在 macOS/Linux 上存在中优先级兼容性问题：

1. **数据目录硬编码为相对路径**：`./data/icp_results.db`、`./data/history.db`、`./hash_store`、`./screenshots` 等
2. **系统级安装失败**：Linux 用户将二进制安装到 `/usr/local/bin/` 后，相对路径 `./data/` 尝试在 `/usr/local/bin/data/` 创建文件，通常无写权限
3. **macOS Homebrew 同理**：安装到 `/opt/homebrew/bin/` 时权限不足

## 决策

1. **新建 `internal/utils/path.go`**：提供 `AppDataDir()`、`HashStoreDir()`、`ScreenshotsDir()`、`DefaultConfigPath()` 等跨平台路径函数
2. **查找顺序**：`os.UserConfigDir()` → `os.UserHomeDir()/.unimap/` → `./data/`（fallback）
3. **所有生产代码**：将硬编码 `./data/` 和 `./hash_store` 迁移为 `utils` 函数调用
4. **测试适配**：更新断言值和默认值以适配跨平台路径
5. **不破坏现有行为**：开发环境仍可通过 `configs/config.yaml` 运行，配置值优先于默认值

## 影响

### 正面影响
- Linux/macOS 系统级安装后可直接运行，无需手动创建目录
- 各平台数据文件存放于符合惯例的位置（XDG/macOS App Support/Windows %APPDATA%）
- 代码可维护性提升：路径逻辑集中管理

### 负面影响
- 首次运行时数据目录位置与之前不同，用户可能需要迁移已有数据
- 测试断言需要适配跨平台路径值

## 实施步骤

1. ✅ 创建 `internal/utils/path.go`
2. ✅ 更新 `internal/config/config_defaults.go`
3. ✅ 更新 `web/server.go` 中 8 处硬编码路径
4. ✅ 更新其他生产代码（tamper、screenshot、service、cmd）
5. ✅ 更新测试断言
6. ✅ 验证 `go build ./...` 和 `go test ./...`

## 相关文件

- `internal/utils/path.go` — 跨平台路径辅助函数
- `internal/config/config_defaults.go` — 默认值迁移
- `web/server.go` — 数据路径集中替换
- `cmd/unimap-gui/main.go` / `cmd/unimap-cli/main.go` / `cmd/unimap-web/main.go` — 配置路径

## 验证

- `go build ./...` 通过
- `go test ./...` 零失败
