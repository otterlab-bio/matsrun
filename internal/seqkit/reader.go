// Package seqkit derives rMATS read length from native SeqKit statistics or
// fastqcx's embedded SeqKit-compatible summary.
package seqkit

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Format identifies the QC report layout used to derive read length.
type Format string

const (
	// FormatAuto accepts exactly one discoverable report layout and rejects a mixed directory.
	FormatAuto Format = "auto"
	// FormatSeqKit parses native *_seqkit_stat.txt TSV files.
	FormatSeqKit Format = "seqkit"
	// FormatFastqcx parses *_fastqcx/fastqc_data.txt Seqkit Statistics modules.
	FormatFastqcx Format = "fastqcx"
)

// ParseFormat converts a CLI value to a supported report layout.
func ParseFormat(value string) (Format, error) {
	switch Format(strings.ToLower(strings.TrimSpace(value))) {
	case FormatAuto:
		return FormatAuto, nil
	case FormatSeqKit:
		return FormatSeqKit, nil
	case FormatFastqcx:
		return FormatFastqcx, nil
	default:
		return "", fmt.Errorf("read-length QC format %q must be one of: auto, seqkit, fastqcx", value)
	}
}

// ComputeReadLength automatically discovers one supported QC report layout.
// It rejects directories containing both layouts so a read length is never silently double-counted.
func ComputeReadLength(dir string) (int, error) {
	return ComputeReadLengthWithFormat(dir, FormatAuto)
}

// ComputeReadLengthWithFormat derives round(mean(N50)) from the requested QC report layout.
func ComputeReadLengthWithFormat(dir string, format Format) (int, error) {
	seqkitFiles, err := filepath.Glob(filepath.Join(dir, "*_seqkit_stat.txt"))
	if err != nil {
		return 0, fmt.Errorf("seqkit: glob native reports: %w", err)
	}
	fastqcxFiles, err := filepath.Glob(filepath.Join(dir, "*_fastqcx", "fastqc_data.txt"))
	if err != nil {
		return 0, fmt.Errorf("seqkit: glob fastqcx reports: %w", err)
	}

	switch format {
	case FormatAuto:
		if len(seqkitFiles) > 0 && len(fastqcxFiles) > 0 {
			return 0, fmt.Errorf("read-length QC: found both native SeqKit and fastqcx reports in %q; specify --seqlength-qc-format", dir)
		}
		if len(seqkitFiles) > 0 {
			return computeNativeSeqKit(seqkitFiles)
		}
		if len(fastqcxFiles) > 0 {
			return computeFastqcx(fastqcxFiles)
		}
		return 0, noReportsError(dir)
	case FormatSeqKit:
		if len(seqkitFiles) == 0 {
			return 0, fmt.Errorf("seqkit: no *_seqkit_stat.txt files found in %q", dir)
		}
		return computeNativeSeqKit(seqkitFiles)
	case FormatFastqcx:
		if len(fastqcxFiles) == 0 {
			return 0, fmt.Errorf("fastqcx: no *_fastqcx/fastqc_data.txt files found in %q", dir)
		}
		return computeFastqcx(fastqcxFiles)
	default:
		return 0, fmt.Errorf("read-length QC format %q is unsupported", format)
	}
}

func noReportsError(dir string) error {
	return fmt.Errorf("read-length QC: no native *_seqkit_stat.txt or fastqcx *_fastqcx/fastqc_data.txt reports found in %q", dir)
}

func computeNativeSeqKit(paths []string) (int, error) {
	var n50Values []float64
	for _, path := range paths {
		values, err := parseSeqKitFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warn: seqkit: skipping %q: %v\n", path, err)
			continue
		}
		n50Values = append(n50Values, values...)
	}
	return meanN50(n50Values, "seqkit")
}

func computeFastqcx(paths []string) (int, error) {
	var n50Values []float64
	for _, path := range paths {
		values, err := parseFastqcxFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warn: fastqcx: skipping %q: %v\n", path, err)
			continue
		}
		n50Values = append(n50Values, values...)
	}
	return meanN50(n50Values, "fastqcx")
}

func meanN50(values []float64, source string) (int, error) {
	if len(values) == 0 {
		return 0, fmt.Errorf("%s: no valid N50 values found", source)
	}
	var sum float64
	for _, value := range values {
		sum += value
	}
	return int(math.Round(sum / float64(len(values)))), nil
}

// parseSeqKitFile reads one SeqKit TSV and returns N50 values for trimmed reads
// (rows whose file column contains "_val_").
func parseSeqKitFile(path string) ([]float64, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true

	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	fileIndex, n50Index := -1, -1
	for index, value := range header {
		switch strings.TrimSpace(value) {
		case "file":
			fileIndex = index
		case "N50":
			n50Index = index
		}
	}
	if fileIndex == -1 || n50Index == -1 {
		return nil, fmt.Errorf("missing 'file' or 'N50' column")
	}

	var values []float64
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		if fileIndex >= len(row) || n50Index >= len(row) || !strings.Contains(row[fileIndex], "_val_") {
			continue
		}
		value, err := strconv.ParseFloat(strings.TrimSpace(row[n50Index]), 64)
		if err == nil {
			values = append(values, value)
		}
	}
	return values, nil
}

// parseFastqcxFile reads the Seqkit Statistics module from one fastqcx
// fastqc_data.txt summary. The summary directory is the selected QC scope, so
// this parser does not infer trimming from the source filename.
func parseFastqcxFile(path string) ([]float64, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	inModule := false
	foundModule := false
	headerSeen := false
	n50Index := -1
	var values []float64

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !inModule {
			if strings.HasPrefix(line, ">>Seqkit Statistics") {
				inModule = true
				foundModule = true
			}
			continue
		}
		if strings.HasPrefix(line, ">>END_MODULE") {
			break
		}
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			if headerSeen {
				return nil, fmt.Errorf("duplicate Seqkit Statistics header")
			}
			headerSeen = true
			fields := strings.Fields(strings.TrimPrefix(line, "#"))
			fileIndex := -1
			for index, field := range fields {
				switch field {
				case "file":
					fileIndex = index
				case "N50":
					n50Index = index
				}
			}
			if fileIndex == -1 || n50Index == -1 {
				return nil, fmt.Errorf("missing 'file' or 'N50' column in Seqkit Statistics module")
			}
			continue
		}
		if !headerSeen {
			return nil, fmt.Errorf("Seqkit Statistics data appeared before its header")
		}
		fields := strings.Fields(line)
		if n50Index >= len(fields) {
			return nil, fmt.Errorf("Seqkit Statistics row has no N50 value")
		}
		value, err := strconv.ParseFloat(fields[n50Index], 64)
		if err != nil {
			return nil, fmt.Errorf("parse Seqkit Statistics N50 %q: %w", fields[n50Index], err)
		}
		values = append(values, value)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if !foundModule {
		return nil, fmt.Errorf("missing Seqkit Statistics module")
	}
	if !headerSeen {
		return nil, fmt.Errorf("missing Seqkit Statistics header")
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("Seqkit Statistics module has no data rows")
	}
	return values, nil
}
