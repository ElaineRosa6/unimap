package screenshot

import (
	"fmt"
	"path/filepath"
	"strings"
)

// sanitizeFilename 清理文件名中的危险字符
func sanitizeFilename(name string) string {
	// 替换所有可能的路径遍历字符
	replacer := strings.NewReplacer(
		"../", "",
		"..\\", "",
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
		"\x00", "",
	)
	clean := replacer.Replace(name)

	// 移除开头的点（防止隐藏文件）
	clean = strings.TrimLeft(clean, ".")

	// 限制长度
	if len(clean) > 200 {
		clean = clean[:200]
	}

	// 确保文件名不为空
	if clean == "" {
		clean = "unnamed"
	}

	return clean
}

// validatePath 验证路径是否在允许的基础目录内
func validatePath(baseDir, targetPath string) error {
	// 获取绝对路径
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute base path: %w", err)
	}

	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute target path: %w", err)
	}

	// 检查目标路径是否在基础目录内（避免 C:\base 与 C:\base2 的前缀误判）
	rel, err := filepath.Rel(absBase, absTarget)
	if err != nil {
		return fmt.Errorf("failed to compute relative path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path traversal detected: target path is outside base directory")
	}

	return nil
}

// safeJoinPath 安全地连接路径，防止路径遍历攻击
func safeJoinPath(baseDir string, elems []string) (string, error) {
	// 清理每个路径元素
	cleanElems := make([]string, len(elems))
	for i, e := range elems {
		cleanElems[i] = sanitizeFilename(e)
	}

	// 连接路径
	allElems := append([]string{baseDir}, cleanElems...)
	result := filepath.Join(allElems...)

	// 验证结果路径
	if err := validatePath(baseDir, result); err != nil {
		return "", err
	}

	return result, nil
}
