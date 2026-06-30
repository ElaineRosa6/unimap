package exporter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unimap/project/internal/model"
	"github.com/xuri/excelize/v2"
)

// fullAsset 返回一个字段全部填充的资产样本，用于导出往返校验。
func fullAsset() model.UnifiedAsset {
	return model.UnifiedAsset{
		IP:          "192.168.1.1",
		Port:        443,
		Protocol:    "https",
		Host:        "example.com",
		URL:         "https://example.com",
		Title:       "Example Site",
		Server:      "nginx",
		StatusCode:  200,
		CountryCode: "US",
		Region:      "California",
		City:        "San Francisco",
		ASN:         "AS12345",
		Org:         "Example Org",
		ISP:         "Example ISP",
		Source:      "shodan",
	}
}

// expectedExcelHeaders 与 exporter.go 中 Excel 表头保持一致（共 15 列）。
var expectedExcelHeaders = []string{
	"IP", "Port", "Protocol", "Host", "URL", "Title", "Server", "Status Code",
	"Country", "Region", "City", "ASN", "Org", "ISP", "Source",
}

// TestJSONExporter_Export 验证 JSON 导出器能将资产往返写出并读回，内容与输入一致。
func TestJSONExporter_Export(t *testing.T) {
	exporter := NewJSONExporter()

	t.Run("空切片", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "empty.json")
		require.NoError(t, exporter.Export([]model.UnifiedAsset{}, path))

		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.NotEmpty(t, data)

		var got []model.UnifiedAsset
		require.NoError(t, json.Unmarshal(data, &got))
		assert.Empty(t, got)
	})

	t.Run("单个资产", func(t *testing.T) {
		assets := []model.UnifiedAsset{fullAsset()}
		path := filepath.Join(t.TempDir(), "single.json")
		require.NoError(t, exporter.Export(assets, path))

		data, err := os.ReadFile(path)
		require.NoError(t, err)

		var got []model.UnifiedAsset
		require.NoError(t, json.Unmarshal(data, &got))
		assert.Equal(t, assets, got)
	})

	t.Run("多个资产字段混合", func(t *testing.T) {
		// 一个完整资产 + 一个仅部分字段填充，覆盖字段为空与为非空的分支。
		assets := []model.UnifiedAsset{
			fullAsset(),
			{
				IP:     "10.0.0.1",
				Port:   80,
				URL:    "http://10.0.0.1",
				Source: "fofa",
				// 其余字段保持零值
			},
		}
		path := filepath.Join(t.TempDir(), "multiple.json")
		require.NoError(t, exporter.Export(assets, path))

		data, err := os.ReadFile(path)
		require.NoError(t, err)

		var got []model.UnifiedAsset
		require.NoError(t, json.Unmarshal(data, &got))
		assert.Equal(t, assets, got)
	})

	t.Run("路径不存在返回错误", func(t *testing.T) {
		// 指向不存在目录时应返回创建文件错误。
		path := filepath.Join(t.TempDir(), "no-such-dir", "out.json")
		err := exporter.Export([]model.UnifiedAsset{fullAsset()}, path)
		require.Error(t, err)
	})
}

// TestExcelExporter_Export 验证 Excel 导出器生成的文件可被 excelize 正确读取，
// 工作表名为 "Assets"，表头共 15 列，且数据行与输入一致。
func TestExcelExporter_Export(t *testing.T) {
	exporter := NewExcelExporter()

	t.Run("多个资产", func(t *testing.T) {
		assets := []model.UnifiedAsset{
			fullAsset(),
			{
				IP:          "10.0.0.1",
				Port:        80,
				Protocol:    "http",
				Host:        "test.local",
				URL:         "http://test.local",
				Title:       "Test Page",
				Server:      "apache",
				StatusCode:  301,
				CountryCode: "CN",
				Region:      "Beijing",
				City:        "Beijing",
				ASN:         "AS67890",
				Org:         "Test Org",
				ISP:         "Test ISP",
				Source:      "fofa",
			},
		}
		path := filepath.Join(t.TempDir(), "assets.xlsx")
		require.NoError(t, exporter.Export(assets, path))

		// 文件存在且非空
		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.False(t, info.IsDir())
		assert.Greater(t, info.Size(), int64(0))

		f, err := excelize.OpenFile(path)
		require.NoError(t, err)
		defer func() { _ = f.Close() }()

		// 校验工作表名称
		assert.Contains(t, f.GetSheetList(), "Assets")

		// 校验表头共 15 列且内容正确
		require.Len(t, expectedExcelHeaders, 15)
		for i, want := range expectedExcelHeaders {
			cell, cellErr := excelize.CoordinatesToCellName(i+1, 1)
			require.NoError(t, cellErr)
			got, getErr := f.GetCellValue("Assets", cell)
			require.NoError(t, getErr)
			assert.Equal(t, want, got, "表头第 %d 列应为 %s", i+1, want)
		}

		// 校验数据行与输入一致（整型字段以字符串形式比对）
		for i, asset := range assets {
			row := i + 2 // 第 1 行为表头，数据从第 2 行开始
			wantCells := map[string]string{
				"A": asset.IP,
				"B": strconv.Itoa(asset.Port),
				"C": asset.Protocol,
				"D": asset.Host,
				"E": asset.URL,
				"F": asset.Title,
				"G": asset.Server,
				"H": strconv.Itoa(asset.StatusCode),
				"I": asset.CountryCode,
				"J": asset.Region,
				"K": asset.City,
				"L": asset.ASN,
				"M": asset.Org,
				"N": asset.ISP,
				"O": asset.Source,
			}
			for col, want := range wantCells {
				got, cellErr := f.GetCellValue("Assets", fmt.Sprintf("%s%d", col, row))
				require.NoError(t, cellErr)
				assert.Equal(t, want, got, "第 %d 行 %s 列", row, col)
			}
		}
	})

	t.Run("空切片仅写表头", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "empty.xlsx")
		require.NoError(t, exporter.Export([]model.UnifiedAsset{}, path))

		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.Greater(t, info.Size(), int64(0))

		f, err := excelize.OpenFile(path)
		require.NoError(t, err)
		defer func() { _ = f.Close() }()

		assert.Contains(t, f.GetSheetList(), "Assets")

		// 仅有表头，无数据行
		for i, want := range expectedExcelHeaders {
			cell, cellErr := excelize.CoordinatesToCellName(i+1, 1)
			require.NoError(t, cellErr)
			got, getErr := f.GetCellValue("Assets", cell)
			require.NoError(t, getErr)
			assert.Equal(t, want, got, "空表头第 %d 列", i+1)
		}

		// 第 2 行应无数据
		got, lastErr := f.GetCellValue("Assets", "A2")
		require.NoError(t, lastErr)
		assert.Empty(t, got)
	})
}
