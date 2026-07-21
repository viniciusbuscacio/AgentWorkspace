package office

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// maxXlsxCells caps how many cells XlsxText renders so a huge workbook cannot
// blow up the agent context; the output notes the truncation.
const maxXlsxCells = 20000

// XlsxSheet is one sheet of a workbook to create.
type XlsxSheet struct {
	Name string
	Rows [][]string
}

// XlsxText renders every sheet as tab-separated rows under a "## Sheet:"
// heading.
func XlsxText(data []byte) (string, error) {
	file, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("could not open XLSX: %w", err)
	}
	defer file.Close()
	var b strings.Builder
	cells := 0
	for _, sheet := range file.GetSheetList() {
		rows, err := file.GetRows(sheet)
		if err != nil {
			return "", fmt.Errorf("read sheet %q: %w", sheet, err)
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("## Sheet: " + sheet + "\n")
		for _, row := range rows {
			if cells += len(row); cells > maxXlsxCells {
				b.WriteString(fmt.Sprintf("[truncated after %d cells]\n", maxXlsxCells))
				return b.String(), nil
			}
			b.WriteString(strings.Join(row, "\t") + "\n")
		}
	}
	return b.String(), nil
}

// ReplaceTextInXlsx replaces oldText with newText inside cell values across
// all sheets. Formula cells are skipped (replacing their cached value would
// silently destroy the formula).
func ReplaceTextInXlsx(data []byte, oldText, newText string) ([]byte, int, error) {
	file, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, 0, fmt.Errorf("could not open XLSX: %w", err)
	}
	defer file.Close()
	total := 0
	for _, sheet := range file.GetSheetList() {
		rows, err := file.GetRows(sheet)
		if err != nil {
			return nil, 0, fmt.Errorf("read sheet %q: %w", sheet, err)
		}
		for rowIndex, row := range rows {
			for colIndex, value := range row {
				if !strings.Contains(value, oldText) {
					continue
				}
				cell, err := excelize.CoordinatesToCellName(colIndex+1, rowIndex+1)
				if err != nil {
					return nil, 0, err
				}
				if formula, _ := file.GetCellFormula(sheet, cell); formula != "" {
					continue
				}
				if err := file.SetCellValue(sheet, cell, cellValue(strings.ReplaceAll(value, oldText, newText))); err != nil {
					return nil, 0, err
				}
				total += strings.Count(value, oldText)
			}
		}
	}
	if total == 0 {
		return nil, 0, nil
	}
	var out bytes.Buffer
	if err := file.Write(&out); err != nil {
		return nil, 0, err
	}
	return out.Bytes(), total, nil
}

// NewXlsx builds a workbook with the given sheets and rows.
func NewXlsx(sheets []XlsxSheet) ([]byte, error) {
	if len(sheets) == 0 {
		return nil, fmt.Errorf("at least one sheet is required")
	}
	file := excelize.NewFile()
	defer file.Close()
	for i, sheet := range sheets {
		name := strings.TrimSpace(sheet.Name)
		if name == "" {
			name = fmt.Sprintf("Sheet%d", i+1)
		}
		if i == 0 {
			if err := file.SetSheetName(file.GetSheetName(0), name); err != nil {
				return nil, err
			}
		} else if _, err := file.NewSheet(name); err != nil {
			return nil, err
		}
		for rowIndex, row := range sheet.Rows {
			values := make([]any, len(row))
			for colIndex, raw := range row {
				values[colIndex] = cellValue(raw)
			}
			cell, err := excelize.CoordinatesToCellName(1, rowIndex+1)
			if err != nil {
				return nil, err
			}
			if err := file.SetSheetRow(name, cell, &values); err != nil {
				return nil, err
			}
		}
	}
	var out bytes.Buffer
	if err := file.Write(&out); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// cellValue stores numeric-looking strings as numbers so spreadsheets stay
// calculable, everything else as text.
func cellValue(raw string) any {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed != raw {
		return raw
	}
	if number, err := strconv.ParseFloat(trimmed, 64); err == nil {
		return number
	}
	return raw
}
