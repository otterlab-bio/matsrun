package bam_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/otterlab-bio/matsrun/internal/bam"
)

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		t.Fatalf("touch %q: %v", path, err)
	}
}

func TestScan_NormalMode(t *testing.T) {
	dir := t.TempDir()
	// Create BAM files: <sampleid>_<species>.bam
	touch(t, filepath.Join(dir, "S1_hg38.bam"))
	touch(t, filepath.Join(dir, "S2_hg38.bam"))
	touch(t, filepath.Join(dir, "S3_hg38_Filtered.bam")) // should be skipped in normal mode

	pdataMap := map[string]string{
		"S1": "Ctrl",
		"S2": "Treat",
	}

	records, err := bam.Scan(dir, false, pdataMap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("got %d records, want 2", len(records))
	}
	for _, r := range records {
		if r.Species != "hg38" {
			t.Errorf("unexpected species %q", r.Species)
		}
	}
}

func TestScan_PDXMode(t *testing.T) {
	root := t.TempDir()
	filteredDir := filepath.Join(root, "Filtered_bams")
	if err := os.MkdirAll(filteredDir, 0755); err != nil {
		t.Fatal(err)
	}
	touch(t, filepath.Join(filteredDir, "patient_01_fixed_human_Filtered.bam"))
	touch(t, filepath.Join(filteredDir, "patient_02_fixed_mouse_Filtered.bam"))

	pdataMap := map[string]string{
		"patient_01": "Ctrl",
		"patient_02": "Treat",
	}

	records, err := bam.Scan(root, true, pdataMap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("got %d records, want 2", len(records))
	}
	if records[0].SampleID != "patient_01" || records[0].Species != "human" {
		t.Errorf("first record = sample %q species %q, want patient_01/human", records[0].SampleID, records[0].Species)
	}
	if records[1].SampleID != "patient_02" || records[1].Species != "mouse" {
		t.Errorf("second record = sample %q species %q, want patient_02/mouse", records[1].SampleID, records[1].Species)
	}
}

func TestScan_SampleIDWithUnderscores(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "patient_01_human.bam"))
	touch(t, filepath.Join(dir, "patient_02_fixed_human.bam"))

	pdataMap := map[string]string{
		"patient_01": "Ctrl",
		"patient_02": "Treat",
	}

	records, err := bam.Scan(dir, false, pdataMap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("got %d records, want 2", len(records))
	}
	for recordIndex, expectedSampleID := range []string{"patient_01", "patient_02"} {
		if records[recordIndex].SampleID != expectedSampleID {
			t.Errorf("record %d sample ID = %q, want %q", recordIndex, records[recordIndex].SampleID, expectedSampleID)
		}
		if records[recordIndex].Species != "human" {
			t.Errorf("record %d species = %q, want human", recordIndex, records[recordIndex].Species)
		}
	}
}

func TestScan_FixedSuffix(t *testing.T) {
	dir := t.TempDir()
	// _fixed suffix should be stripped before splitting
	touch(t, filepath.Join(dir, "S1_hg38_fixed.bam"))

	pdataMap := map[string]string{"S1": "Ctrl"}

	records, err := bam.Scan(dir, false, pdataMap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	if records[0].Species != "hg38" {
		t.Errorf("species = %q, want hg38", records[0].Species)
	}
}

func TestScan_UnmatchedSampleReturnsError(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "S99_hg38.bam"))

	pdataMap := map[string]string{"S1": "Ctrl"}

	if _, err := bam.Scan(dir, false, pdataMap); err == nil {
		t.Fatal("expected unmatched BAM sample to return an error")
	}
}

func TestScan_MissingPdataSampleReturnsError(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "S1_hg38.bam"))

	pdataMap := map[string]string{
		"S1": "Ctrl",
		"S2": "Treat",
	}

	if _, err := bam.Scan(dir, false, pdataMap); err == nil {
		t.Fatal("expected missing BAM for pdata sample to return an error")
	}
}
