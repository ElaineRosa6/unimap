package utils

import (
	"fmt"
	"os"
	"path/filepath"
)

// AppDataDir 返回跨平台的应用数据目录。
//
// 查找顺序：
//  1. os.UserConfigDir()/unimap/<subPaths...>
//  2. os.UserHomeDir()/.unimap/<subPaths...>
//  3. ./data/<subPaths...>（最后的 fallback，用于开发环境）
//
// 各平台对应路径：
//   - Linux:   ~/.config/unimap/<subPaths...>
//   - macOS:   ~/Library/Application Support/unimap/<subPaths...>
//   - Windows: %APPDATA%\unimap\<subPaths...>
func AppDataDir(subPaths ...string) string {
	base := filepath.Join("data", filepath.Join(subPaths...))

	// 优先使用 XDG / macOS Application Support / Windows %APPDATA%
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		candidate := filepath.Join(dir, "unimap", filepath.Join(subPaths...))
		if isWritable(candidate) {
			return candidate
		}
	}

	// 次选：用户主目录下的 .unimap
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		candidate := filepath.Join(home, ".unimap", filepath.Join(subPaths...))
		if isWritable(candidate) {
			return candidate
		}
	}

	// 最后 fallback 到相对路径（开发环境兼容）
	return base
}

// isWritable 检查目录是否存在且可写（不创建目录）。
func isWritable(dir string) bool {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return false
	}
	// 通过尝试创建临时文件验证写权限
	f, err := os.CreateTemp(dir, ".writetest")
	if f != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
	}
	return err == nil
}

// EnsureDataDir 确保数据目录存在，返回实际路径。
func EnsureDataDir(subPaths ...string) string {
	dir := AppDataDir(subPaths...)
	_ = os.MkdirAll(dir, 0755)
	return dir
}

// DefaultDataPath 返回默认数据文件的完整路径。
func DefaultDataPath(filename string) string {
	return filepath.Join(AppDataDir(), filename)
}

// ScreenshotsDir 返回截图存储目录的跨平台默认路径。
func ScreenshotsDir() string {
	return AppDataDir("screenshots")
}

// HashStoreDir 返回篡改检测哈希存储目录的跨平台默认路径。
func HashStoreDir() string {
	return AppDataDir("hash_store")
}

// DataDir 返回数据根目录的跨平台默认路径。
func DataDir() string {
	return AppDataDir()
}

// RelDataPath 将相对路径中的 ./data/ 前缀替换为跨平台数据目录。
// 例如: "./data/icp_results.db" → "~/.config/unimap/icp_results.db"
// 若 relPath 不以 ./data/ 开头，则直接返回原始路径。
func RelDataPath(relPath string) string {
	prefix := "./data/"
	if relPath == prefix || relPath == "data/" {
		return DefaultDataPath(relPath[len(prefix)-1:])
	}
	if len(relPath) > len(prefix) && relPath[:len(prefix)] == prefix {
		return filepath.Join(AppDataDir(), relPath[len(prefix)-1:])
	}
	// 无 ./data/ 前缀，按原样返回
	return relPath
}

// ConfigPath 返回跨平台的默认配置文件路径。
// 优先查找 configs/config.yaml（项目相对路径，用于开发），
// 若不可用则回退到用户配置目录。
func DefaultConfigPath() string {
	// 开发环境优先：项目相对路径
	if _, err := os.Stat("configs/config.yaml"); err == nil {
		return "configs/config.yaml"
	}

	// 生产环境：用户配置目录
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		return filepath.Join(dir, "unimap", "config.yaml")
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".unimap", "config.yaml")
	}

	return "configs/config.yaml"
}

// AppDataDirWithErr 返回跨平台应用数据目录及可选的错误信息。
func AppDataDirWithErr(subPaths ...string) (string, error) {
	dir := AppDataDir(subPaths...)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return dir, fmt.Errorf("create app data dir %s: %w", dir, err)
	}
	return dir, nil
}
