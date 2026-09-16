// Package types defines all data structures used by matsrun.
// Per Type-First convention, all types are declared here before any business logic.
package types

// SampleRecord is a BAM file joined with pdata metadata.
type SampleRecord struct {
	SampleID    string // pdata "sampleid" column
	SampleGroup string // pdata "sample_group" column
	BamPath     string // absolute path to BAM file
	Species     string // species/type token parsed from BAM filename
}

// GroupCombination represents one pairwise group contrast.
type GroupCombination struct {
	Group1 string `json:"group1"`
	Group2 string `json:"group2"`
}

// ContrastManifestSchemaVersion identifies the on-disk contrast manifest contract.
const ContrastManifestSchemaVersion = "matsrun.contrast-manifest/v1"

// ExpectedProduct defines one required rMATS output and its minimum header contract.
type ExpectedProduct struct {
	RelativePath    string   `json:"relative_path"`
	RequiredColumns []string `json:"required_columns"`
}

// ContrastManifest records the exact inputs and command used for one published contrast.
type ContrastManifest struct {
	SchemaVersion    string            `json:"schema_version"`
	Species          string            `json:"species"`
	Combination      GroupCombination  `json:"combination"`
	B1Paths          []string          `json:"b1_paths"`
	B2Paths          []string          `json:"b2_paths"`
	GTFFile          string            `json:"gtf_file"`
	ReadLength       int               `json:"read_length"`
	Threads          int               `json:"threads"`
	RMATSVersion     string            `json:"rmats_version"`
	Command          []string          `json:"command"`
	ExpectedProducts []ExpectedProduct `json:"expected_products"`
}

// ContrastTask is a single rmats.py execution unit (one species × one contrast).
type ContrastTask struct {
	Species     string
	Combination GroupCombination
	B1Paths     []string
	B2Paths     []string
	OutputDir   string // {root}/RNASplicing/{species}/{g1}_vs_{g2}/
	TempDir     string // {root}/RNASplicing/{species}/{g1}_vs_{g2}/temp/
}

// RunConfig holds the complete configuration parsed from CLI flags.
type RunConfig struct {
	RootDir     string
	Threads     int
	PdataFile   string
	SeqLengthQC string
	GTFFile     string
	PDXMode     bool
}
