package exporter

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unimap/project/internal/tamper"
	"github.com/xuri/excelize/v2"
)

// formatTS 与源码使用相同的时间格式，确保期望值与实现一致（受本地时区影响，故同侧计算）。
func formatTS(ts int64) string {
	return time.Unix(ts, 0).Format("2006-01-02 15:04:05")
}

// TestConvertToExportFormat 表驱动覆盖 convertToExportFormat 的全部分支：
// 可达性状态、篡改标记、基线时间有无、检测时间格式化、空输入。
func TestConvertToExportFormat(t *testing.T) {
	const (
		checkTS    = int64(1700000000) // 检测时间戳
		baselineTS = int64(1600000000) // 基线时间戳
	)

	checkTime := formatTS(checkTS)
	baselineTime := formatTS(baselineTS)

	tests := []struct {
		name  string
		input tamper.TamperCheckResult
		want  TamperExportResult
	}{
		{
			name: "状态ok可达",
			input: tamper.TamperCheckResult{
				URL:       "http://ok.example.com",
				Status:    "ok",
				Timestamp: checkTS,
			},
			want: TamperExportResult{
				URL:          "http://ok.example.com",
				Reachable:    "是",
				Screenshot:   "",
				Tampered:     "否",
				BaselineTime: "",
				CheckTime:    checkTime,
			},
		},
		{
			name: "状态success可达",
			input: tamper.TamperCheckResult{
				URL:       "http://success.example.com",
				Status:    "success",
				Timestamp: checkTS,
			},
			want: TamperExportResult{
				URL:          "http://success.example.com",
				Reachable:    "是",
				Tampered:     "否",
				BaselineTime: "",
				CheckTime:    checkTime,
			},
		},
		{
			name: "状态unreachable不可达",
			input: tamper.TamperCheckResult{
				URL:       "http://down.example.com",
				Status:    "unreachable",
				Timestamp: checkTS,
			},
			want: TamperExportResult{
				URL:          "http://down.example.com",
				Reachable:    "否",
				Tampered:     "否",
				BaselineTime: "",
				CheckTime:    checkTime,
			},
		},
		{
			name: "状态failed不可达",
			input: tamper.TamperCheckResult{
				URL:       "http://fail.example.com",
				Status:    "failed",
				Timestamp: checkTS,
			},
			want: TamperExportResult{
				URL:          "http://fail.example.com",
				Reachable:    "否",
				Tampered:     "否",
				BaselineTime: "",
				CheckTime:    checkTime,
			},
		},
		{
			name: "已篡改",
			input: tamper.TamperCheckResult{
				URL:       "http://tampered.example.com",
				Status:    "ok",
				Tampered:  true,
				Timestamp: checkTS,
			},
			want: TamperExportResult{
				URL:          "http://tampered.example.com",
				Reachable:    "是",
				Tampered:     "是",
				BaselineTime: "",
				CheckTime:    checkTime,
			},
		},
		{
			name: "未篡改显式false",
			input: tamper.TamperCheckResult{
				URL:       "http://safe.example.com",
				Status:    "ok",
				Tampered:  false,
				Timestamp: checkTS,
			},
			want: TamperExportResult{
				URL:          "http://safe.example.com",
				Reachable:    "是",
				Tampered:     "否",
				BaselineTime: "",
				CheckTime:    checkTime,
			},
		},
		{
			name: "有基线时间",
			input: tamper.TamperCheckResult{
				URL: "http://baseline.example.com",
				BaselineHash: &tamper.PageHashResult{
					Timestamp: baselineTS,
				},
				Status:    "ok",
				Timestamp: checkTS,
			},
			want: TamperExportResult{
				URL:          "http://baseline.example.com",
				Reachable:    "是",
				Tampered:     "否",
				BaselineTime: baselineTime,
				CheckTime:    checkTime,
			},
		},
		{
			name: "无基线时间",
			input: tamper.TamperCheckResult{
				URL:       "http://nobaseline.example.com",
				Status:    "ok",
				Timestamp: checkTS,
			},
			want: TamperExportResult{
				URL:          "http://nobaseline.example.com",
				Reachable:    "是",
				Tampered:     "否",
				BaselineTime: "",
				CheckTime:    checkTime,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &TamperJSONExporter{}
			got := e.convertToExportFormat([]tamper.TamperCheckResult{tt.input})
			require.Len(t, got, 1)
			assert.Equal(t, tt.want, got[0])
		})
	}

	t.Run("空输入返回空输出", func(t *testing.T) {
		e := &TamperJSONExporter{}
		got := e.convertToExportFormat([]tamper.TamperCheckResult{})
		assert.Empty(t, got)
	})

	// TamperExcelExporter 复用同一转换逻辑，确保与 JSON 导出器结果一致。
	t.Run("excel导出器转换结果与json一致", func(t *testing.T) {
		results := []tamper.TamperCheckResult{
			{
				URL: "http://a.example.com",
				BaselineHash: &tamper.PageHashResult{
					Timestamp: baselineTS,
				},
				Status:    "unreachable",
				Tampered:  true,
				Timestamp: checkTS,
			},
		}
		jsonRes := (&TamperJSONExporter{}).convertToExportFormat(results)
		excelRes := (&TamperExcelExporter{}).convertToExportFormat(results)
		assert.Equal(t, jsonRes, excelRes)
	})
}

// TestTamperJSONExporter_Export 验证篡改检测结果导出为 JSON 后能正确读回。
func TestTamperJSONExporter_Export(t *testing.T) {
	exporter := NewTamperJSONExporter()

	t.Run("多个结果", func(t *testing.T) {
		results := []tamper.TamperCheckResult{
			{
				URL:       "http://a.example.com",
				Status:    "ok",
				Tampered:  false,
				Timestamp: 1700000000,
			},
			{
				URL: "http://b.example.com",
				BaselineHash: &tamper.PageHashResult{
					Timestamp: 1600000000,
				},
				Status:    "unreachable",
				Tampered:  true,
				Timestamp: 1700000001,
			},
		}

		path := filepath.Join(t.TempDir(), "tamper.json")
		require.NoError(t, exporter.Export(results, path))

		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.NotEmpty(t, data)

		var got []TamperExportResult
		require.NoError(t, json.Unmarshal(data, &got))
		require.Len(t, got, len(results))

		// 第一项：可达、未篡改、无基线
		assert.Equal(t, "http://a.example.com", got[0].URL)
		assert.Equal(t, "是", got[0].Reachable)
		assert.Equal(t, "否", got[0].Tampered)
		assert.Empty(t, got[0].BaselineTime)
		assert.Equal(t, formatTS(1700000000), got[0].CheckTime)

		// 第二项：不可达、已篡改、有基线
		assert.Equal(t, "http://b.example.com", got[1].URL)
		assert.Equal(t, "否", got[1].Reachable)
		assert.Equal(t, "是", got[1].Tampered)
		assert.Equal(t, formatTS(1600000000), got[1].BaselineTime)
		assert.Equal(t, formatTS(1700000001), got[1].CheckTime)
	})

	t.Run("空结果", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "tamper_empty.json")
		require.NoError(t, exporter.Export([]tamper.TamperCheckResult{}, path))

		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.Greater(t, info.Size(), int64(0))

		data, err := os.ReadFile(path)
		require.NoError(t, err)

		var got []TamperExportResult
		require.NoError(t, json.Unmarshal(data, &got))
		assert.Empty(t, got)
	})
}

// expectedTamperExcelHeaders 与 tamper_exporter.go 中 Excel 表头保持一致（共 6 列）。
var expectedTamperExcelHeaders = []string{"URL", "可达性", "网页截图", "是否被篡改", "基线时间", "检测时间"}

// TestTamperExcelExporter_Export 验证篡改检测结果导出为 Excel 后文件存在且非空，
// 并可被 excelize 正确读取，工作表名与表头符合预期。
func TestTamperExcelExporter_Export(t *testing.T) {
	exporter := NewTamperExcelExporter()

	t.Run("多个结果", func(t *testing.T) {
		results := []tamper.TamperCheckResult{
			{
				URL:       "http://a.example.com",
				Status:    "ok",
				Tampered:  false,
				Timestamp: 1700000000,
			},
			{
				URL: "http://b.example.com",
				BaselineHash: &tamper.PageHashResult{
					Timestamp: 1600000000,
				},
				Status:    "unreachable",
				Tampered:  true,
				Timestamp: 1700000001,
			},
		}

		path := filepath.Join(t.TempDir(), "tamper.xlsx")
		require.NoError(t, exporter.Export(results, path))

		// 文件存在且非空
		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.False(t, info.IsDir())
		assert.Greater(t, info.Size(), int64(0))

		f, err := excelize.OpenFile(path)
		require.NoError(t, err)
		defer func() { _ = f.Close() }()

		// 校验工作表名称
		assert.Contains(t, f.GetSheetList(), "Tamper Detection")

		// 校验表头（6 列）
		require.Len(t, expectedTamperExcelHeaders, 6)
		for i, want := range expectedTamperExcelHeaders {
			cell, cellErr := excelize.CoordinatesToCellName(i+1, 1)
			require.NoError(t, cellErr)
			got, getErr := f.GetCellValue("Tamper Detection", cell)
			require.NoError(t, getErr)
			assert.Equal(t, want, got, "表头第 %d 列应为 %s", i+1, want)
		}

		// 校验首行数据
		a2, a2Err := f.GetCellValue("Tamper Detection", "A2")
		require.NoError(t, a2Err)
		assert.Equal(t, "http://a.example.com", a2)
		b2, b2Err := f.GetCellValue("Tamper Detection", "B2")
		require.NoError(t, b2Err)
		assert.Equal(t, "是", b2)
		d2, d2Err := f.GetCellValue("Tamper Detection", "D2")
		require.NoError(t, d2Err)
		assert.Equal(t, "否", d2)
	})

	t.Run("空结果", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "tamper_empty.xlsx")
		require.NoError(t, exporter.Export([]tamper.TamperCheckResult{}, path))

		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.Greater(t, info.Size(), int64(0))

		f, err := excelize.OpenFile(path)
		require.NoError(t, err)
		defer func() { _ = f.Close() }()

		assert.Contains(t, f.GetSheetList(), "Tamper Detection")
	})
}
