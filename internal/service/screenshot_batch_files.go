package service

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// BatchInfo 批次信息
type BatchInfo struct {
	Name      string `json:"name"`
	FileCount int    `json:"file_count"`
	UpdatedAt int64  `json:"updated_at"`
}

// FileInfo 文件信息
type FileInfo struct {
	Name       string `json:"name"`
	Size       int64  `json:"size"`
	UpdatedAt  int64  `json:"updated_at"`
	PreviewURL string `json:"preview_url,omitempty"`
}

// ListBatches 列出所有截图批次
func (s *ScreenshotAppService) ListBatches() ([]BatchInfo, error) {
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []BatchInfo{}, nil
		}
		return nil, fmt.Errorf("failed to read screenshot directory: %w", err)
	}

	batches := make([]BatchInfo, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			continue
		}

		fileCount := 0
		children, childErr := os.ReadDir(filepath.Join(s.baseDir, entry.Name()))
		if childErr == nil {
			for _, child := range children {
				if !child.IsDir() {
					fileCount++
				}
			}
		}

		batches = append(batches, BatchInfo{
			Name:      entry.Name(),
			FileCount: fileCount,
			UpdatedAt: info.ModTime().Unix(),
		})
	}

	sort.Slice(batches, func(i, j int) bool {
		return batches[i].UpdatedAt > batches[j].UpdatedAt
	})

	return batches, nil
}

// ListBatchFiles 列出指定批次的文件
func (s *ScreenshotAppService) ListBatchFiles(batch string, previewURLBuilder func(string) string) ([]FileInfo, error) {
	batchToken := s.normalizePathToken(batch)
	if batchToken == "" {
		return nil, fmt.Errorf("invalid batch name")
	}

	batchDir := filepath.Join(s.baseDir, batchToken)
	absBatchDir, err := filepath.Abs(batchDir)
	if err != nil {
		return nil, fmt.Errorf("invalid batch path")
	}

	// 安全检查：确保目录在 baseDir 内
	absBaseDir, err := filepath.Abs(s.baseDir)
	if err != nil {
		return nil, fmt.Errorf("invalid base directory")
	}
	rel, err := filepath.Rel(absBaseDir, absBatchDir)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return nil, fmt.Errorf("invalid batch path")
	}

	entries, err := os.ReadDir(absBatchDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("batch not found")
		}
		return nil, fmt.Errorf("failed to read batch directory: %w", err)
	}

	files := make([]FileInfo, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			continue
		}

		absPath := filepath.Join(absBatchDir, entry.Name())
		previewURL := ""
		if previewURLBuilder != nil {
			previewURL = previewURLBuilder(absPath)
		}

		files = append(files, FileInfo{
			Name:       entry.Name(),
			Size:       info.Size(),
			UpdatedAt:  info.ModTime().Unix(),
			PreviewURL: previewURL,
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].UpdatedAt > files[j].UpdatedAt
	})

	return files, nil
}

// DeleteBatch 删除指定批次
func (s *ScreenshotAppService) DeleteBatch(batch string) error {
	batchToken := s.normalizePathToken(batch)
	if batchToken == "" {
		return fmt.Errorf("invalid batch name")
	}

	batchDir := filepath.Join(s.baseDir, batchToken)
	absBatchDir, err := filepath.Abs(batchDir)
	if err != nil {
		return fmt.Errorf("invalid batch path")
	}

	// 安全检查
	absBaseDir, err := filepath.Abs(s.baseDir)
	if err != nil {
		return fmt.Errorf("invalid base directory")
	}
	rel, err := filepath.Rel(absBaseDir, absBatchDir)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return fmt.Errorf("invalid batch path")
	}

	if _, err := os.Stat(absBatchDir); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("batch not found")
		}
		return fmt.Errorf("failed to access batch: %w", err)
	}

	return os.RemoveAll(absBatchDir)
}

// DeleteFile 删除指定批次中的文件
func (s *ScreenshotAppService) DeleteFile(batch, fileName string) error {
	batchToken := s.normalizePathToken(batch)
	if batchToken == "" {
		return fmt.Errorf("invalid batch name")
	}

	fileToken := s.normalizePathToken(fileName)
	if fileToken == "" {
		return fmt.Errorf("invalid file name")
	}

	batchDir := filepath.Join(s.baseDir, batchToken)
	absBatchDir, err := filepath.Abs(batchDir)
	if err != nil {
		return fmt.Errorf("invalid batch path")
	}

	// 安全检查
	absBaseDir, err := filepath.Abs(s.baseDir)
	if err != nil {
		return fmt.Errorf("invalid base directory")
	}
	rel, err := filepath.Rel(absBaseDir, absBatchDir)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return fmt.Errorf("invalid batch path")
	}

	// Remove old hash entries

	targetFile := filepath.Join(absBatchDir, fileToken)
	absTarget, err := filepath.Abs(targetFile)
	if err != nil {
		return fmt.Errorf("invalid file path")
	}

	// 安全检查：确保文件在批次目录内
	relFile, err := filepath.Rel(absBatchDir, absTarget)
	if err != nil || relFile == "." || strings.HasPrefix(relFile, "..") {
		return fmt.Errorf("invalid file path")
	}

	info, err := os.Stat(absTarget)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file not found")
		}
		return fmt.Errorf("failed to access file: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("file name points to a directory")
	}

	return os.Remove(absTarget)
}

// normalizePathToken 规范化路径令牌，防止路径穿越
func (s *ScreenshotAppService) normalizePathToken(raw string) string {
	token := strings.TrimSpace(raw)
	if token == "" || token == "." || token == ".." {
		return ""
	}
	if strings.Contains(token, "/") || strings.Contains(token, "\\") {
		return ""
	}
	if filepath.Base(token) != token {
		return ""
	}
	return token
}
