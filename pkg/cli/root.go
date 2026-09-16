package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var Version = "0.1.0"

var rootCmd = &cobra.Command{
	Use:   "matsrun",
	Short: "Go rMATS orchestrator for RNA splicing analysis",
	Long: `matsrun orchestrates rMATS pairwise splicing comparisons.

It reads a pdata Excel file, scans BAM files, computes read length from
seqkit statistics, generates all pairwise group combinations, and executes
rmats.py for each combination × species.

	Example:
	  matsrun run --root /data/bam --pdata samples.xlsx \
	             --seqlengthQC /data/qc --gtf hg38.gtf`,
	Version: Version,
}

// Execute is the entry point called from main.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.SetVersionTemplate("matsrun {{.Version}}\n")
}
