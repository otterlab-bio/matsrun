package seqkit_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/otterlab-bio/matsrun/internal/seqkit"
)

// writeStatFile creates a temporary seqkit_stat.txt TSV file.
func writeStatFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatalf("writeStatFile: %v", err)
	}
}

func TestComputeReadLength_Basic(t *testing.T) {
	dir := t.TempDir()

	// N50 values: 100 and 120 → mean 110, round → 110
	writeStatFile(t, dir, "s1_seqkit_stat.txt",
		"file\tformat\ttype\tnum_seqs\tsum_len\tmin_len\tavg_len\tmax_len\tQ1\tQ2\tQ3\tsum_gap\tN50\tQ20(%)\tQ30(%)\tgc(%)\n"+
			"/path/s1_val_1.fq.gz\tFASTQ\tDNA\t1000\t100000\t50\t100.0\t150\t90\t100\t110\t0\t100\t95.0\t90.0\t50.0\n"+
			"/path/s1_R1.fq.gz\tFASTQ\tDNA\t1000\t100000\t50\t100.0\t150\t90\t100\t110\t0\t200\t95.0\t90.0\t50.0\n", // not _val_, should be excluded
	)
	writeStatFile(t, dir, "s2_seqkit_stat.txt",
		"file\tformat\ttype\tnum_seqs\tsum_len\tmin_len\tavg_len\tmax_len\tQ1\tQ2\tQ3\tsum_gap\tN50\tQ20(%)\tQ30(%)\tgc(%)\n"+
			"/path/s2_val_1.fq.gz\tFASTQ\tDNA\t1000\t120000\t60\t120.0\t150\t110\t120\t130\t0\t120\t95.0\t90.0\t50.0\n",
	)

	got, err := seqkit.ComputeReadLength(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 110 {
		t.Errorf("got readLength=%d, want 110", got)
	}
}

func TestComputeReadLength_NoValRows(t *testing.T) {
	dir := t.TempDir()
	writeStatFile(t, dir, "s1_seqkit_stat.txt",
		"file\tN50\n/path/s1_R1.fq.gz\t100\n",
	)
	_, err := seqkit.ComputeReadLength(dir)
	if err == nil {
		t.Fatal("expected error when no _val_ rows, got nil")
	}
}

func TestComputeReadLength_NoFiles(t *testing.T) {
	dir := t.TempDir()
	_, err := seqkit.ComputeReadLength(dir)
	if err == nil {
		t.Fatal("expected error for empty directory, got nil")
	}
}
