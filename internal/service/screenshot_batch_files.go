package service

import (
	"fmt"
	"io/fs"
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
	root, err := os.OpenRoot(s.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []BatchInfo{}, nil
		}
		return nil, fmt.Errorf("failed to open screenshot directory: %w", err)
	}
	defer root.Close()
	entries, err := fs.ReadDir(root.FS(), ".")
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
		info, infoErr := root.Lstat(entry.Name())
		if infoErr != nil {
			continue
		}

		fileCount := 0
		children, childErr := fs.ReadDir(root.FS(), entry.Name())
		if childErr == nil {
			for _, child := range children {
				if child.Type().IsRegular() {
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

	root, err := os.OpenRoot(s.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("batch not found: %w", err)
		}
		return nil, fmt.Errorf("failed to open screenshot directory: %w", err)
	}
	defer root.Close()
	batchRoot, err := root.OpenRoot(batchToken)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("batch not found: %w", err)
		}
		return nil, fmt.Errorf("failed to open batch directory: %w", err)
	}
	defer batchRoot.Close()
	absBatchDir, err := filepath.Abs(filepath.Join(s.baseDir, batchToken))
	if err != nil {
		return nil, fmt.Errorf("invalid batch path: %w", err)
	}
	entries, err := fs.ReadDir(batchRoot.FS(), ".")
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("batch not found")
		}
		return nil, fmt.Errorf("failed to read batch directory: %w", err)
	}

	files := make([]FileInfo, 0)
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}
		info, infoErr := batchRoot.Lstat(entry.Name())
		if infoErr != nil || !info.Mode().IsRegular() {
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

	root, err := os.OpenRoot(s.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("batch not found: %w", err)
		}
		return fmt.Errorf("failed to open screenshot directory: %w", err)
	}
	defer root.Close()
	info, err := root.Stat(batchToken)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("batch not found: %w", err)
		}
		return fmt.Errorf("failed to access batch: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("batch name does not point to a directory")
	}
	// Resolve and remove relative to the same directory handle, not a checked
	// absolute path that could be redirected by a concurrent symlink change.
	return root.RemoveAll(batchToken)
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

	root, err := os.OpenRoot(s.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file not found: %w", err)
		}
		return fmt.Errorf("failed to open screenshot directory: %w", err)
	}
	defer root.Close()
	batchRoot, err := root.OpenRoot(batchToken)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file not found: %w", err)
		}
		return fmt.Errorf("failed to open batch directory: %w", err)
	}
	defer batchRoot.Close()
	info, err := batchRoot.Stat(fileToken)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file not found: %w", err)
		}
		return fmt.Errorf("failed to access file: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("file name points to a directory")
	}
	return batchRoot.Remove(fileToken)
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
