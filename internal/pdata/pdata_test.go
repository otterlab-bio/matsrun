package pdata

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/360EntSecGroup-Skylar/excelize"
)

func writePdataWorkbook(t *testing.T, rows [][]interface{}) string {
	t.Helper()
	workbook := excelize.NewFile()
	for rowIndex, row := range rows {
		for columnIndex, value := range row {
			cell := fmt.Sprintf("%c%d", 'A'+columnIndex, rowIndex+1)
			workbook.SetCellValue("Sheet1", cell, value)
		}
	}
	path := filepath.Join(t.TempDir(), "pdata.xlsx")
	if err := workbook.SaveAs(path); err != nil {
		t.Fatalf("save pdata workbook: %v", err)
	}
	return path
}

func TestLoadUsesConditionWhenSampleGroupIsAbsent(t *testing.T) {
	path := writePdataWorkbook(t, [][]interface{}{
		{"sampleid", "condition"},
		{"S1", "Control"},
		{"S2", "Treatment"},
	})

	groups, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if groups["S1"] != "Control" || groups["S2"] != "Treatment" {
		t.Fatalf("unexpected groups: %#v", groups)
	}
}

func TestLoadRejectsDuplicateSampleID(t *testing.T) {
	path := writePdataWorkbook(t, [][]interface{}{
		{"sampleid", "sample_group"},
		{"S1", "Control"},
		{"S1", "Treatment"},
	})

	if _, err := Load(path); err == nil {
		t.Fatal("expected duplicate sample ID error")
	}
}

func TestLoadRejectsEmptyGroup(t *testing.T) {
	path := writePdataWorkbook(t, [][]interface{}{
		{"sampleid", "sample_group"},
		{"S1", ""},
	})

	if _, err := Load(path); err == nil {
		t.Fatal("expected empty sample group error")
	}
}
