package exporter

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/unimap/project/internal/logger"
	"github.com/unimap/project/internal/model"
	"github.com/xuri/excelize/v2"
)

// Exporter 导出器接口
type Exporter interface {
	Export(assets []model.UnifiedAsset, filepath string) error
}

// JSONExporter JSON导出器
type JSONExporter struct{}

// NewJSONExporter 创建JSON导出器
func NewJSONExporter() *JSONExporter {
	return &JSONExporter{}
}

// Export 导出为JSON文件
func (e *JSONExporter) Export(assets []model.UnifiedAsset, filepath string) error {
	file, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(assets); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	return nil
}

// ExcelExporter Excel导出器
type ExcelExporter struct{}

// NewExcelExporter 创建Excel导出器
func NewExcelExporter() *ExcelExporter {
	return &ExcelExporter{}
}

// Export 导出为Excel文件
func (e *ExcelExporter) Export(assets []model.UnifiedAsset, filepath string) error {
	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			logger.Warnf("Failed to close Excel file: %v", err)
		}
	}()

	sheetName := "Assets"
	index, err := f.NewSheet(sheetName)
	if err != nil {
		return fmt.Errorf("failed to create sheet: %w", err)
	}

	// setCell 写入单元格值，忽略不可恢复的错误（excelize 在写入普通值时极少失败）
	setCell := func(cell string, value interface{}) {
		_ = f.SetCellValue(sheetName, cell, value)
	}

	// 设置表头
	headers := []string{"IP", "Port", "Protocol", "Host", "URL", "Title", "Server", "Status Code", "Country", "Region", "City", "ASN", "Org", "ISP", "Source"}
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		setCell(cell, header)
	}

	// 写入数据
	for i, asset := range assets {
		row := i + 2 // 从第2行开始（第1行是表头）
		setCell(fmt.Sprintf("A%d", row), asset.IP)
		setCell(fmt.Sprintf("B%d", row), asset.Port)
		setCell(fmt.Sprintf("C%d", row), asset.Protocol)
		setCell(fmt.Sprintf("D%d", row), asset.Host)
		setCell(fmt.Sprintf("E%d", row), asset.URL)
		setCell(fmt.Sprintf("F%d", row), asset.Title)
		setCell(fmt.Sprintf("G%d", row), asset.Server)
		setCell(fmt.Sprintf("H%d", row), asset.StatusCode)
		setCell(fmt.Sprintf("I%d", row), asset.CountryCode)
		setCell(fmt.Sprintf("J%d", row), asset.Region)
		setCell(fmt.Sprintf("K%d", row), asset.City)
		setCell(fmt.Sprintf("L%d", row), asset.ASN)
		setCell(fmt.Sprintf("M%d", row), asset.Org)
		setCell(fmt.Sprintf("N%d", row), asset.ISP)
		setCell(fmt.Sprintf("O%d", row), asset.Source)
	}

	// 设置默认活动工作表
	f.SetActiveSheet(index)

	// 保存文件
	if err := f.SaveAs(filepath); err != nil {
		return fmt.Errorf("failed to save Excel file: %w", err)
	}

	return nil
}

// ExportFull 导出为全量字段 Excel：覆盖 UnifiedAsset 的全部标准字段，并将
// Headers 与各资产 Extra 扩展字段展开为独立列（Extra 键并集，前缀 extra. 防撞名）。
// 标准 Export 仅写 15 个基础列，本方法保留全部引擎字段，供字段不丢失回放使用。
func (e *ExcelExporter) ExportFull(assets []model.UnifiedAsset, filepath string) error {
	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			logger.Warnf("Failed to close Excel file: %v", err)
		}
	}()

	sheetName := "Assets"
	index, err := f.NewSheet(sheetName)
	if err != nil {
		return fmt.Errorf("failed to create sheet: %w", err)
	}

	setCell := func(cell string, value interface{}) {
		_ = f.SetCellValue(sheetName, cell, value)
	}

	extraKeys := collectExtraKeys(assets)
	headers := fullExportHeaders(extraKeys)
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		setCell(cell, header)
	}

	for i, asset := range assets {
		row := i + 2 // 第1行是表头
		values := fullExportRow(asset, extraKeys)
		for j, value := range values {
			cell, _ := excelize.CoordinatesToCellName(j+1, row)
			setCell(cell, value)
		}
	}

	f.SetActiveSheet(index)
	if err := f.SaveAs(filepath); err != nil {
		return fmt.Errorf("failed to save Excel file: %w", err)
	}
	return nil
}

// collectExtraKeys 返回所有资产 Extra 键的排序并集。
func collectExtraKeys(assets []model.UnifiedAsset) []string {
	set := make(map[string]struct{})
	for i := range assets {
		for k := range assets[i].Extra {
			set[k] = struct{}{}
		}
	}
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func fullExportHeaders(extraKeys []string) []string {
	headers := []string{
		"IP", "Port", "Protocol", "Host", "URL", "Title", "BodySnippet",
		"Server", "StatusCode", "CountryCode", "Region", "City", "ASN",
		"Org", "ISP", "LastSeen", "Source", "Headers",
	}
	for _, k := range extraKeys {
		headers = append(headers, "extra."+k)
	}
	return headers
}

func fullExportRow(asset model.UnifiedAsset, extraKeys []string) []interface{} {
	headersJSON := "{}"
	if len(asset.Headers) > 0 {
		if b, err := json.Marshal(asset.Headers); err == nil {
			headersJSON = string(b)
		}
	}
	values := []interface{}{
		asset.IP, asset.Port, asset.Protocol, asset.Host, asset.URL,
		asset.Title, asset.BodySnippet, asset.Server, asset.StatusCode,
		asset.CountryCode, asset.Region, asset.City, asset.ASN,
		asset.Org, asset.ISP, asset.LastSeen, asset.Source, headersJSON,
	}
	for _, k := range extraKeys {
		values = append(values, extraCellValue(asset.Extra[k]))
	}
	return values
}

// extraCellValue 将 Extra 值转换为 Excel 单元格可写值：标量直接写，嵌套结构
// （map/slice）序列化为紧凑 JSON，nil 写空串。
func extraCellValue(v interface{}) interface{} {
	if v == nil {
		return ""
	}
	switch v.(type) {
	case string, bool, int, int32, int64, float32, float64, json.Number:
		return v
	default:
		if b, err := json.Marshal(v); err == nil {
			return string(b)
		}
		return fmt.Sprintf("%v", v)
	}
}
