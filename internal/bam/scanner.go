// Package bam scans BAM files and joins them with pdata metadata.
package bam

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/otterlab-bio/matsrun/internal/types"
)

// Scan discovers BAM files under rootDir, strips naming suffixes, and
// left-joins with pdataMap (sampleid → sample_group).
//
// Normal mode: *.bam in rootDir (excluding *_Filtered.bam)
// PDX mode:    *_Filtered.bam in rootDir/Filtered_bams/
//
// Unmatched BAM files emit a warning to stderr and are skipped.
func Scan(rootDir string, pdxMode bool, pdataMap map[string]string) ([]types.SampleRecord, error) {
	var pattern string
	var bamDir string

	if pdxMode {
		bamDir = filepath.Join(rootDir, "Filtered_bams")
		pattern = filepath.Join(bamDir, "*_Filtered.bam")
	} else {
		bamDir = rootDir
		pattern = filepath.Join(bamDir, "*.bam")
	}

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("bam: glob %q: %w", pattern, err)
	}

	records := make([]types.SampleRecord, 0, len(matches))
	observedSamples := make(map[string]struct{}, len(pdataMap))
	observedSampleSpecies := make(map[string]string, len(matches))

	for _, bamPath := range matches {
		absolutePath, absolutePathError := filepath.Abs(bamPath)
		if absolutePathError != nil {
			return nil, fmt.Errorf("bam: resolve absolute path for %q: %w", bamPath, absolutePathError)
		}

		baseName := filepath.Base(bamPath)
		if !pdxMode && strings.HasSuffix(baseName, "_Filtered.bam") {
			continue
		}

		nameWithoutSuffix := strings.TrimSuffix(baseName, "_Filtered.bam")
		nameWithoutSuffix = strings.TrimSuffix(nameWithoutSuffix, ".bam")
		nameWithoutSuffix = strings.TrimSuffix(nameWithoutSuffix, "_fixed")

		separatorIndex := strings.LastIndex(nameWithoutSuffix, "_")
		if separatorIndex <= 0 || separatorIndex == len(nameWithoutSuffix)-1 {
			return nil, fmt.Errorf("bam: cannot parse sampleid/species from %q", baseName)
		}

		sampleID := strings.TrimSuffix(nameWithoutSuffix[:separatorIndex], "_fixed")
		species := nameWithoutSuffix[separatorIndex+1:]
		if sampleID == "" || species == "" {
			return nil, fmt.Errorf("bam: cannot parse sampleid/species from %q", baseName)
		}

		sampleGroup, exists := pdataMap[sampleID]
		if !exists {
			return nil, fmt.Errorf("bam: sampleid %q from %q is not present in pdata", sampleID, baseName)
		}

		sampleSpeciesKey := sampleID + "\x00" + species
		if previousPath, duplicate := observedSampleSpecies[sampleSpeciesKey]; duplicate {
			return nil, fmt.Errorf(
				"bam: duplicate BAMs for sample %q species %q: %q and %q",
				sampleID,
				species,
				previousPath,
				absolutePath,
			)
		}
		observedSampleSpecies[sampleSpeciesKey] = absolutePath
		observedSamples[sampleID] = struct{}{}

		records = append(records, types.SampleRecord{
			SampleID:    sampleID,
			SampleGroup: sampleGroup,
			BamPath:     absolutePath,
			Species:     species,
		})
	}

	missingSamples := make([]string, 0)
	for sampleID := range pdataMap {
		if _, observed := observedSamples[sampleID]; !observed {
			missingSamples = append(missingSamples, sampleID)
		}
	}
	if len(missingSamples) > 0 {
		sort.Strings(missingSamples)
		return nil, fmt.Errorf("bam: no BAM file found for pdata sample(s): %s", strings.Join(missingSamples, ", "))
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("bam: no BAM files matched %q", pattern)
	}

	return records, nil
}
