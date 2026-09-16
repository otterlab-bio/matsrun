// Package seqkit parses *_seqkit_stat.txt files to compute mean read length.
package seqkit

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ComputeReadLength scans dir for *_seqkit_stat.txt files, collects N50
// values for trimmed reads (those whose "file" column contains "_val_"),
// and returns round(mean(N50)).  Returns an error when no valid data is
// found.
func ComputeReadLength(dir string) (int, error) {
	pattern := filepath.Join(dir, "*_seqkit_stat.txt")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return 0, fmt.Errorf("seqkit: glob %q: %w", pattern, err)
	}
	if len(files) == 0 {
		return 0, fmt.Errorf("seqkit: no *_seqkit_stat.txt files found in %q", dir)
	}

	var n50Values []float64

	for _, path := range files {
		vals, err := parseFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warn: seqkit: skipping %q: %v\n", path, err)
			continue
		}
		n50Values = append(n50Values, vals...)
	}

	if len(n50Values) == 0 {
		return 0, fmt.Errorf("seqkit: no valid N50 values found in %q", dir)
	}

	var sum float64
	for _, v := range n50Values {
		sum += v
	}
	mean := sum / float64(len(n50Values))
	return int(math.Round(mean)), nil
}

// parseFile reads one seqkit_stat TSV and returns N50 values for rows
// whose "file" column contains "_val_".
func parseFile(path string) ([]float64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.Comma = '\t'
	r.LazyQuotes = true

	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	fileIdx, n50Idx := -1, -1
	for i, h := range header {
		switch strings.TrimSpace(h) {
		case "file":
			fileIdx = i
		case "N50":
			n50Idx = i
		}
	}
	if fileIdx == -1 || n50Idx == -1 {
		return nil, fmt.Errorf("missing 'file' or 'N50' column")
	}

	var result []float64
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		if fileIdx >= len(row) || n50Idx >= len(row) {
			continue
		}
		if !strings.Contains(row[fileIdx], "_val_") {
			continue
		}
		raw := strings.TrimSpace(row[n50Idx])
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			// N/A or non-numeric — skip
			continue
		}
		result = append(result, v)
	}

	return result, nil
}
