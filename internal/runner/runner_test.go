package runner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/otterlab-bio/matsrun/internal/types"
)

func TestRunRejectsEmptyGroupsWithoutCreatingOutput(t *testing.T) {
	task := newTestTask(t)
	task.B1Paths = nil

	err := Run(task, "/reference/annotations.gtf", 150, 4)
	if err == nil {
		t.Fatal("Run should reject an empty comparison group")
	}
	if _, statErr := os.Stat(task.OutputDir); !os.IsNotExist(statErr) {
		t.Fatalf("invalid task created output directory, stat error: %v", statErr)
	}
}

func TestRunPublishesValidatedProductsAndManifest(t *testing.T) {
	installFakeRMATS(t)
	t.Setenv("FAKE_RMATS_MODE", "success")
	t.Setenv("FAKE_RMATS_RUN_TAG", "first-run")
	task := newTestTask(t)

	if err := Run(task, "/reference/annotations.gtf", 150, 4); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	assertFileContent(t, filepath.Join(task.OutputDir, "b1.txt"), "/data/sample1.bam\n")
	assertFileContent(t, filepath.Join(task.OutputDir, "b2.txt"), "/data/sample2.bam\n")
	assertFileContent(t, filepath.Join(task.OutputDir, "run-tag.txt"), "first-run\n")
	if _, err := os.Stat(task.TempDir); !os.IsNotExist(err) {
		t.Fatalf("published output should not retain the rMATS temp directory, stat error: %v", err)
	}
	for _, product := range requiredRMATSProducts {
		assertFileContent(
			t,
			filepath.Join(task.OutputDir, product.RelativePath),
			"ID\tGeneID\tFDR\n",
		)
	}

	manifestBytes, err := os.ReadFile(filepath.Join(task.OutputDir, manifestFileName))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest types.ContrastManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	if manifest.SchemaVersion != types.ContrastManifestSchemaVersion {
		t.Fatalf("manifest schema = %q, want %q", manifest.SchemaVersion, types.ContrastManifestSchemaVersion)
	}
	if manifest.RMATSVersion != "rmats.py 4.1.2-test" {
		t.Fatalf("manifest rMATS version = %q", manifest.RMATSVersion)
	}
	if len(manifest.Command) == 0 || manifest.Command[0] != "rmats.py" {
		t.Fatalf("manifest command was not recorded: %#v", manifest.Command)
	}
	if len(manifest.ExpectedProducts) != len(requiredRMATSProducts) {
		t.Fatalf("manifest expected products = %d, want %d", len(manifest.ExpectedProducts), len(requiredRMATSProducts))
	}
	assertNoTransactionArtifacts(t, task.OutputDir)
}

func TestRunRejectsSuccessfulCommandWithMissingProduct(t *testing.T) {
	installFakeRMATS(t)
	t.Setenv("FAKE_RMATS_MODE", "missing-product")
	task := newTestTask(t)

	err := Run(task, "/reference/annotations.gtf", 150, 4)
	if err == nil || !strings.Contains(err.Error(), "RI.MATS.JC.txt") {
		t.Fatalf("Run error = %v, want missing RI product error", err)
	}
	if _, statErr := os.Stat(task.OutputDir); !os.IsNotExist(statErr) {
		t.Fatalf("invalid result was published, stat error: %v", statErr)
	}
	assertNoTransactionArtifacts(t, task.OutputDir)
}

func TestRunRejectsProductWithInvalidHeader(t *testing.T) {
	installFakeRMATS(t)
	t.Setenv("FAKE_RMATS_MODE", "bad-header")
	task := newTestTask(t)

	err := Run(task, "/reference/annotations.gtf", 150, 4)
	if err == nil || !strings.Contains(err.Error(), "FDR") {
		t.Fatalf("Run error = %v, want missing FDR column error", err)
	}
	if _, statErr := os.Stat(task.OutputDir); !os.IsNotExist(statErr) {
		t.Fatalf("invalid result was published, stat error: %v", statErr)
	}
	assertNoTransactionArtifacts(t, task.OutputDir)
}

func TestRunCommandFailurePreservesPreviousOutput(t *testing.T) {
	installFakeRMATS(t)
	t.Setenv("FAKE_RMATS_MODE", "command-failure")
	task := newTestTask(t)
	if err := os.MkdirAll(task.OutputDir, 0o755); err != nil {
		t.Fatalf("create previous output: %v", err)
	}
	previousOutputPath := filepath.Join(task.OutputDir, "previous-result.txt")
	if err := os.WriteFile(previousOutputPath, []byte("previous\n"), 0o644); err != nil {
		t.Fatalf("write previous output: %v", err)
	}

	if err := Run(task, "/reference/annotations.gtf", 150, 4); err == nil {
		t.Fatal("Run should return the fake rMATS command failure")
	}
	assertFileContent(t, previousOutputPath, "previous\n")
	if _, err := os.Stat(filepath.Join(task.OutputDir, "b1.txt")); !os.IsNotExist(err) {
		t.Fatalf("failed run modified the published output, stat error: %v", err)
	}
	assertNoTransactionArtifacts(t, task.OutputDir)
}

func TestRunRerunAtomicallyReplacesPreviousOutput(t *testing.T) {
	installFakeRMATS(t)
	t.Setenv("FAKE_RMATS_MODE", "success")
	t.Setenv("FAKE_RMATS_RUN_TAG", "first-run")
	task := newTestTask(t)

	if err := Run(task, "/reference/annotations.gtf", 150, 4); err != nil {
		t.Fatalf("first Run returned error: %v", err)
	}
	firstOnlyPath := filepath.Join(task.OutputDir, "first-only.txt")
	if err := os.WriteFile(firstOnlyPath, []byte("stale\n"), 0o644); err != nil {
		t.Fatalf("write stale output marker: %v", err)
	}

	t.Setenv("FAKE_RMATS_RUN_TAG", "second-run")
	if err := Run(task, "/reference/annotations.gtf", 150, 4); err != nil {
		t.Fatalf("second Run returned error: %v", err)
	}
	assertFileContent(t, filepath.Join(task.OutputDir, "run-tag.txt"), "second-run\n")
	if _, err := os.Stat(firstOnlyPath); !os.IsNotExist(err) {
		t.Fatalf("rerun retained stale output, stat error: %v", err)
	}
	assertNoTransactionArtifacts(t, task.OutputDir)
}

func TestRunRejectsBAMPathsThatBreakRMATSListFormat(t *testing.T) {
	task := newTestTask(t)
	task.B1Paths = []string{"/data/sample,one.bam"}

	err := Run(task, "/reference/annotations.gtf", 150, 4)
	if err == nil || !strings.Contains(err.Error(), "delimiter") {
		t.Fatalf("Run error = %v, want unsupported delimiter error", err)
	}
	if _, statErr := os.Stat(task.OutputDir); !os.IsNotExist(statErr) {
		t.Fatalf("invalid task created output directory, stat error: %v", statErr)
	}
}

func TestRunRejectsSymlinkedPublishedOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink behavior requires Unix test permissions")
	}
	installFakeRMATS(t)
	t.Setenv("FAKE_RMATS_MODE", "success")
	task := newTestTask(t)
	externalDirectory := t.TempDir()
	if err := os.MkdirAll(filepath.Dir(task.OutputDir), 0o755); err != nil {
		t.Fatalf("create output parent: %v", err)
	}
	if err := os.Symlink(externalDirectory, task.OutputDir); err != nil {
		t.Fatalf("create output symlink: %v", err)
	}

	err := Run(task, "/reference/annotations.gtf", 150, 4)
	if err == nil || !strings.Contains(err.Error(), "real directory") {
		t.Fatalf("Run error = %v, want symlink rejection", err)
	}
	if entries, readErr := os.ReadDir(externalDirectory); readErr != nil || len(entries) != 0 {
		t.Fatalf("external symlink target was modified: entries=%v error=%v", entries, readErr)
	}
	assertNoTransactionArtifacts(t, task.OutputDir)
}

func newTestTask(t *testing.T) types.ContrastTask {
	t.Helper()
	outputDirectory := filepath.Join(t.TempDir(), "RNASplicing", "human", "contrast-001")
	return types.ContrastTask{
		Species: "human",
		Combination: types.GroupCombination{
			Group1: "control",
			Group2: "treatment",
		},
		B1Paths:   []string{"/data/sample1.bam"},
		B2Paths:   []string{"/data/sample2.bam"},
		OutputDir: outputDirectory,
		TempDir:   filepath.Join(outputDirectory, "temp"),
	}
}

func installFakeRMATS(t *testing.T) {
	t.Helper()
	binaryDirectory := t.TempDir()
	binaryPath := filepath.Join(binaryDirectory, "rmats.py")
	script := `#!/usr/bin/env sh
if [ "${1:-}" = "--version" ]; then
    printf '%s\n' 'rmats.py 4.1.2-test'
    exit 0
fi
output_directory=''
while [ "$#" -gt 0 ]; do
    case "$1" in
        --od)
            output_directory="$2"
            shift 2
            ;;
        *)
            shift
            ;;
    esac
done
if [ -z "$output_directory" ]; then
    exit 9
fi
if [ "${FAKE_RMATS_MODE:-success}" = "command-failure" ]; then
    exit 7
fi
mkdir -p "$output_directory"
for product in SE.MATS.JC.txt MXE.MATS.JC.txt A5SS.MATS.JC.txt A3SS.MATS.JC.txt RI.MATS.JC.txt; do
    if [ "${FAKE_RMATS_MODE:-success}" = "missing-product" ] && [ "$product" = "RI.MATS.JC.txt" ]; then
        continue
    fi
    if [ "${FAKE_RMATS_MODE:-success}" = "bad-header" ]; then
        printf 'ID\tGeneID\n' > "$output_directory/$product"
    else
        printf 'ID\tGeneID\tFDR\n' > "$output_directory/$product"
    fi
done
printf '%s\n' "${FAKE_RMATS_RUN_TAG:-default}" > "$output_directory/run-tag.txt"
`
	if err := os.WriteFile(binaryPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake rmats.py: %v", err)
	}
	t.Setenv("PATH", binaryDirectory+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func assertFileContent(t *testing.T, path, expectedContent string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %q: %v", path, err)
	}
	if string(content) != expectedContent {
		t.Fatalf("content of %q = %q, want %q", path, content, expectedContent)
	}
}

func assertNoTransactionArtifacts(t *testing.T, outputDirectory string) {
	t.Helper()
	parentDirectory := filepath.Dir(outputDirectory)
	patterns := []string{
		filepath.Join(parentDirectory, "."+filepath.Base(outputDirectory)+".staging-*"),
		filepath.Join(parentDirectory, "."+filepath.Base(outputDirectory)+".backup-*"),
	}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatalf("glob transaction artifacts with %q: %v", pattern, err)
		}
		if len(matches) != 0 {
			t.Fatalf("transaction artifacts remain for %q: %v", outputDirectory, matches)
		}
	}
}
