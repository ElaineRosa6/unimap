package exporter

import (
	"encoding/json"
	"fmt"
	"os"

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
