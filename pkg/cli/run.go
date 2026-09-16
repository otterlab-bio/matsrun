package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/otterlab-bio/matsrun/internal/bam"
	"github.com/otterlab-bio/matsrun/internal/combinator"
	"github.com/otterlab-bio/matsrun/internal/pdata"
	"github.com/otterlab-bio/matsrun/internal/runner"
	"github.com/otterlab-bio/matsrun/internal/seqkit"
	"github.com/otterlab-bio/matsrun/internal/types"
	"github.com/spf13/cobra"
)

var (
	flagRoot        string
	flagThreads     int
	flagPdata       string
	flagSeqLengthQC string
	flagGTF         string
	flagPDXMode     string
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run rMATS pairwise splicing analysis",
	Long: `Scan BAM files, compute read length from seqkit stats, and execute
rmats.py for every pairwise group combination × species.`,
	RunE: runMain,
	Args: cobra.NoArgs,
}

func init() {
	runCmd.Flags().StringVar(&flagRoot, "root", "", "Root directory containing BAM files (required)")
	runCmd.Flags().IntVar(&flagThreads, "threads", 10, "Number of threads passed to rmats.py")
	runCmd.Flags().StringVar(&flagPdata, "pdata", "", "Path to pdata.xlsx (required)")
	runCmd.Flags().StringVar(&flagSeqLengthQC, "seqlengthQC", "", "Directory containing *_seqkit_stat.txt files (required)")
	runCmd.Flags().StringVar(&flagGTF, "gtf", "", "Path to GTF annotation file (required)")
	runCmd.Flags().StringVar(&flagPDXMode, "pdxmode", "0", `PDX mode: "1" = scan Filtered_bams/, "0" = scan root dir`)

	runCmd.MarkFlagRequired("root")
	runCmd.MarkFlagRequired("pdata")
	runCmd.MarkFlagRequired("seqlengthQC")
	runCmd.MarkFlagRequired("gtf")

	rootCmd.AddCommand(runCmd)
}

func validatePathComponent(label, value string) error {
	if value == "" || value == "." || value == ".." {
		return fmt.Errorf("matsrun: %s %q is not a valid path component", label, value)
	}
	if strings.ContainsAny(value, `/\\`) || filepath.Base(value) != value {
		return fmt.Errorf("matsrun: %s %q must not contain path separators", label, value)
	}
	return nil
}

func contrastDirectories(rootDir, species, taskID string) (string, string, error) {
	if err := validatePathComponent("species", species); err != nil {
		return "", "", err
	}
	if err := validatePathComponent("task ID", taskID); err != nil {
		return "", "", err
	}

	absoluteRootDir, err := filepath.Abs(rootDir)
	if err != nil {
		return "", "", fmt.Errorf("matsrun: resolve root directory %q: %w", rootDir, err)
	}
	outputDir := filepath.Join(absoluteRootDir, "RNASplicing", species, taskID)
	relativeOutputDir, err := filepath.Rel(absoluteRootDir, outputDir)
	if err != nil || relativeOutputDir == ".." || strings.HasPrefix(relativeOutputDir, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("matsrun: task output %q escapes root directory %q", outputDir, absoluteRootDir)
	}
	return outputDir, filepath.Join(outputDir, "temp"), nil
}

func runMain(cmd *cobra.Command, args []string) error {
	if flagThreads <= 0 {
		return fmt.Errorf("matsrun: --threads must be greater than zero")
	}
	if flagPDXMode != "0" && flagPDXMode != "1" {
		return fmt.Errorf("matsrun: --pdxmode must be either 0 or 1")
	}
	pdxMode := flagPDXMode == "1"

	// 2. Load pdata → map[sampleid]group.
	fmt.Fprintf(os.Stderr, ">> matsrun: loading pdata from %q\n", flagPdata)
	pdataMap, err := pdata.Load(flagPdata)
	if err != nil {
		return fmt.Errorf("matsrun: %w", err)
	}
	fmt.Fprintf(os.Stderr, "   loaded %d samples\n", len(pdataMap))

	// 3. Scan BAM files.
	fmt.Fprintf(os.Stderr, ">> matsrun: scanning BAM files in %q (pdxMode=%v)\n", flagRoot, pdxMode)
	records, err := bam.Scan(flagRoot, pdxMode, pdataMap)
	if err != nil {
		return fmt.Errorf("matsrun: %w", err)
	}
	fmt.Fprintf(os.Stderr, "   found %d BAM records\n", len(records))
	if len(records) == 0 {
		return fmt.Errorf("matsrun: no BAM files matched; check --root and --pdxmode")
	}

	// 4. Compute read length from seqkit stats.
	fmt.Fprintf(os.Stderr, ">> matsrun: computing read length from %q\n", flagSeqLengthQC)
	readLength, err := seqkit.ComputeReadLength(flagSeqLengthQC)
	if err != nil {
		return fmt.Errorf("matsrun: %w", err)
	}
	fmt.Fprintf(os.Stderr, "   readLength = %d\n", readLength)

	// 5. Collect unique species.
	speciesSet := make(map[string]struct{})
	for _, r := range records {
		speciesSet[r.Species] = struct{}{}
	}
	speciesList := make([]string, 0, len(speciesSet))
	for s := range speciesSet {
		speciesList = append(speciesList, s)
	}
	sort.Strings(speciesList)

	// 6. Generate pairwise group combinations.
	groupNames := make([]string, 0, len(pdataMap))
	for _, g := range pdataMap {
		groupNames = append(groupNames, g)
	}
	combinations := combinator.Pairwise(groupNames)
	if len(combinations) == 0 {
		return fmt.Errorf("matsrun: at least two non-empty sample groups are required")
	}
	fmt.Fprintf(os.Stderr, ">> matsrun: %d species × %d combinations = %d tasks\n",
		len(speciesList), len(combinations), len(speciesList)*len(combinations))

	// 7. Execute one task per species × combination.
	var errors []error
	for _, species := range speciesList {
		fmt.Fprintf(os.Stderr, ">> matsrun: processing species %q\n", species)

		// Filter records for this species.
		var speciesRecords []types.SampleRecord
		for _, r := range records {
			if r.Species == species {
				speciesRecords = append(speciesRecords, r)
			}
		}

		for combinationIndex, combo := range combinations {
			// Collect BAM paths for each group.
			var b1, b2 []string
			for _, r := range speciesRecords {
				switch r.SampleGroup {
				case combo.Group1:
					b1 = append(b1, r.BamPath)
				case combo.Group2:
					b2 = append(b2, r.BamPath)
				}
			}

			taskID := fmt.Sprintf("contrast-%03d", combinationIndex+1)
			outputDir, tempDir, directoryError := contrastDirectories(flagRoot, species, taskID)
			if directoryError != nil {
				errors = append(errors, directoryError)
				continue
			}

			task := types.ContrastTask{
				Species:     species,
				Combination: combo,
				B1Paths:     b1,
				B2Paths:     b2,
				OutputDir:   outputDir,
				TempDir:     tempDir,
			}

			if err := runner.Run(task, flagGTF, readLength, flagThreads); err != nil {
				errors = append(errors, fmt.Errorf("%s/%s (%s vs %s): %w", species, taskID, combo.Group1, combo.Group2, err))
			}
		}
	}

	// 8. Print summary.
	fmt.Fprintf(os.Stderr, "\n>> matsrun: summary — %d tasks, %d errors\n",
		len(speciesList)*len(combinations), len(errors))
	for _, e := range errors {
		fmt.Fprintf(os.Stderr, "   ERROR: %v\n", e)
	}

	if len(errors) > 0 {
		return fmt.Errorf("matsrun: %d task(s) failed", len(errors))
	}
	return nil
}
