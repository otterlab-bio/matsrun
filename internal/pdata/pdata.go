// Package pdata reads phenotype data (pdata) from Excel files.
package pdata

import (
	"fmt"
	"strings"

	"github.com/360EntSecGroup-Skylar/excelize"
)

// columnAliases maps alternate/Chinese column names to canonical names.
// This table is synchronized with xdxtools/internal/input/pdata.go normalizePDataColumns().
var columnAliases = map[string]string{
	"样本编号":      "sampleid",
	"样本ID":      "sampleid",
	"sample_id": "sampleid",
	"样本分组":      "sample_group",
	"分组":        "sample_group",
	"group":     "sample_group",
	"条件":        "condition",
	"treatment": "condition",
	"condition": "condition",
}

func normalize(columnName string) string {
	trimmedColumnName := strings.TrimSpace(columnName)
	lowerColumnName := strings.ToLower(trimmedColumnName)
	if canonicalName, exists := columnAliases[trimmedColumnName]; exists {
		return canonicalName
	}
	if canonicalName, exists := columnAliases[lowerColumnName]; exists {
		return canonicalName
	}
	return lowerColumnName
}

// Load reads the first sheet of an xlsx file and returns a map of
// sampleid → sample_group.  Returns an error when either required
// column is absent.
func Load(path string) (map[string]string, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("pdata: open %q: %w", path, err)
	}

	sheetMap := f.GetSheetMap()
	if len(sheetMap) == 0 {
		return nil, fmt.Errorf("pdata: %q has no sheets", path)
	}
	sheetName := sheetMap[1]

	rows := f.GetRows(sheetName)
	if len(rows) == 0 {
		return nil, fmt.Errorf("pdata: sheet %q is empty", sheetName)
	}

	// Normalise header row
	header := rows[0]
	normHeader := make([]string, len(header))
	for i, h := range header {
		normHeader[i] = normalize(h)
	}

	sampleIDIndex := -1
	sampleGroupIndex := -1
	conditionIndex := -1
	for columnIndex, columnName := range normHeader {
		switch columnName {
		case "sampleid":
			if sampleIDIndex != -1 {
				return nil, fmt.Errorf("pdata: duplicate sample ID column in %q", path)
			}
			sampleIDIndex = columnIndex
		case "sample_group":
			if sampleGroupIndex != -1 {
				return nil, fmt.Errorf("pdata: duplicate sample_group column in %q", path)
			}
			sampleGroupIndex = columnIndex
		case "condition":
			if conditionIndex != -1 {
				return nil, fmt.Errorf("pdata: duplicate condition column in %q", path)
			}
			conditionIndex = columnIndex
		}
	}
	if sampleIDIndex == -1 {
		return nil, fmt.Errorf("pdata: missing required column 'sampleid' in %q", path)
	}
	groupIndex := sampleGroupIndex
	groupColumnName := "sample_group"
	if groupIndex == -1 {
		groupIndex = conditionIndex
		groupColumnName = "condition"
	}
	if groupIndex == -1 {
		return nil, fmt.Errorf("pdata: missing grouping column 'sample_group' or 'condition' in %q", path)
	}

	result := make(map[string]string, len(rows)-1)
	for rowOffset, row := range rows[1:] {
		rowNumber := rowOffset + 2
		if sampleIDIndex >= len(row) || groupIndex >= len(row) {
			return nil, fmt.Errorf("pdata: row %d is missing sampleid or %s", rowNumber, groupColumnName)
		}
		sampleID := strings.TrimSpace(row[sampleIDIndex])
		sampleGroup := strings.TrimSpace(row[groupIndex])
		if sampleID == "" {
			return nil, fmt.Errorf("pdata: row %d has an empty sampleid", rowNumber)
		}
		if sampleGroup == "" {
			return nil, fmt.Errorf("pdata: row %d sample %q has an empty %s", rowNumber, sampleID, groupColumnName)
		}
		if _, exists := result[sampleID]; exists {
			return nil, fmt.Errorf("pdata: row %d duplicates sampleid %q", rowNumber, sampleID)
		}
		result[sampleID] = sampleGroup
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("pdata: %q contains no sample rows", path)
	}

	return result, nil
}
