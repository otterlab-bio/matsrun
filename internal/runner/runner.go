// Package runner builds, validates, and transactionally publishes rmats.py results.
package runner

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/otterlab-bio/matsrun/internal/types"
)

const manifestFileName = "matsrun-contrast-manifest.json"

var requiredRMATSProducts = []types.ExpectedProduct{
	{RelativePath: "SE.MATS.JC.txt", RequiredColumns: []string{"ID", "GeneID", "FDR"}},
	{RelativePath: "MXE.MATS.JC.txt", RequiredColumns: []string{"ID", "GeneID", "FDR"}},
	{RelativePath: "A5SS.MATS.JC.txt", RequiredColumns: []string{"ID", "GeneID", "FDR"}},
	{RelativePath: "A3SS.MATS.JC.txt", RequiredColumns: []string{"ID", "GeneID", "FDR"}},
	{RelativePath: "RI.MATS.JC.txt", RequiredColumns: []string{"ID", "GeneID", "FDR"}},
}

// Run executes one contrast in an isolated sibling staging directory, validates
// the required rMATS products, and atomically publishes the completed directory.
func Run(task types.ContrastTask, gtf string, readLength, threads int) error {
	label := fmt.Sprintf(
		"%s/%s_vs_%s",
		task.Species,
		task.Combination.Group1,
		task.Combination.Group2,
	)
	if err := validateRunInputs(task, gtf, readLength, threads); err != nil {
		return fmt.Errorf("runner: cannot run %s: %w", label, err)
	}

	outputParent := filepath.Dir(task.OutputDir)
	if err := os.MkdirAll(outputParent, 0o755); err != nil {
		return fmt.Errorf("runner: create output parent %q: %w", outputParent, err)
	}

	stagingDirectory, err := os.MkdirTemp(
		outputParent,
		"."+filepath.Base(task.OutputDir)+".staging-",
	)
	if err != nil {
		return fmt.Errorf("runner: create staging directory for %s: %w", label, err)
	}
	defer func() {
		_ = os.RemoveAll(stagingDirectory)
	}()

	stagingTempDirectory := filepath.Join(stagingDirectory, "temp")
	if err := os.MkdirAll(stagingTempDirectory, 0o755); err != nil {
		return fmt.Errorf("runner: create staging temp directory for %s: %w", label, err)
	}

	b1File := filepath.Join(stagingDirectory, "b1.txt")
	b2File := filepath.Join(stagingDirectory, "b2.txt")
	if err := writeTextFile(b1File, strings.Join(task.B1Paths, ",")+"\n"); err != nil {
		return fmt.Errorf("runner: write b1.txt for %s: %w", label, err)
	}
	if err := writeTextFile(b2File, strings.Join(task.B2Paths, ",")+"\n"); err != nil {
		return fmt.Errorf("runner: write b2.txt for %s: %w", label, err)
	}

	arguments := []string{
		"--b1", b1File,
		"--b2", b2File,
		"--gtf", gtf,
		"-t", "paired",
		"--readLength", fmt.Sprintf("%d", readLength),
		"--variable-read-length",
		"--nthread", fmt.Sprintf("%d", threads),
		"--od", stagingDirectory,
		"--tmp", stagingTempDirectory,
	}
	manifest := types.ContrastManifest{
		SchemaVersion:    types.ContrastManifestSchemaVersion,
		Species:          task.Species,
		Combination:      task.Combination,
		B1Paths:          append([]string(nil), task.B1Paths...),
		B2Paths:          append([]string(nil), task.B2Paths...),
		GTFFile:          gtf,
		ReadLength:       readLength,
		Threads:          threads,
		RMATSVersion:     detectRMATSVersion(),
		Command:          append([]string{"rmats.py"}, arguments...),
		ExpectedProducts: expectedProducts(),
	}
	if err := writeManifest(filepath.Join(stagingDirectory, manifestFileName), manifest); err != nil {
		return fmt.Errorf("runner: write manifest for %s: %w", label, err)
	}

	fmt.Fprintf(os.Stderr, ">> runner: executing rmats.py for %s\n", label)
	command := exec.Command("rmats.py", arguments...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("runner: rmats.py failed for %s: %w", label, err)
	}

	if err := validateProducts(stagingDirectory, manifest.ExpectedProducts); err != nil {
		return fmt.Errorf("runner: validate rMATS products for %s: %w", label, err)
	}
	if err := os.RemoveAll(stagingTempDirectory); err != nil {
		return fmt.Errorf("runner: remove staging temp directory for %s: %w", label, err)
	}
	if err := publishDirectory(stagingDirectory, task.OutputDir); err != nil {
		return fmt.Errorf("runner: publish %s: %w", label, err)
	}

	fmt.Fprintf(os.Stderr, ">> runner: completed %s\n", label)
	return nil
}

func validateRunInputs(task types.ContrastTask, gtf string, readLength, threads int) error {
	if len(task.B1Paths) == 0 || len(task.B2Paths) == 0 {
		return fmt.Errorf(
			"both comparison groups require at least one BAM file (group %q: %d, group %q: %d)",
			task.Combination.Group1,
			len(task.B1Paths),
			task.Combination.Group2,
			len(task.B2Paths),
		)
	}
	if task.OutputDir == "" {
		return errors.New("output directory is empty")
	}
	if filepath.Clean(task.TempDir) != filepath.Join(filepath.Clean(task.OutputDir), "temp") {
		return fmt.Errorf("temp directory %q must be the output directory's temp child", task.TempDir)
	}
	if strings.TrimSpace(gtf) == "" {
		return errors.New("GTF path is empty")
	}
	if readLength <= 0 {
		return fmt.Errorf("read length must be greater than zero, got %d", readLength)
	}
	if threads <= 0 {
		return fmt.Errorf("thread count must be greater than zero, got %d", threads)
	}
	for _, bamPath := range append(append([]string(nil), task.B1Paths...), task.B2Paths...) {
		if strings.ContainsAny(bamPath, ",\r\n") {
			return fmt.Errorf("BAM path %q contains a delimiter unsupported by rMATS list files", bamPath)
		}
	}
	return nil
}

func expectedProducts() []types.ExpectedProduct {
	products := make([]types.ExpectedProduct, len(requiredRMATSProducts))
	for productIndex, product := range requiredRMATSProducts {
		products[productIndex] = types.ExpectedProduct{
			RelativePath:    product.RelativePath,
			RequiredColumns: append([]string(nil), product.RequiredColumns...),
		}
	}
	return products
}

func detectRMATSVersion() string {
	output, err := exec.Command("rmats.py", "--version").CombinedOutput()
	if err != nil {
		return "unavailable"
	}
	version := strings.TrimSpace(string(output))
	if version == "" {
		return "unavailable"
	}
	return version
}

func writeTextFile(path, content string) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := file.WriteString(content); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func writeManifest(path string, manifest types.ContrastManifest) error {
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	manifestBytes = append(manifestBytes, '\n')
	return writeTextFile(path, string(manifestBytes))
}

func validateProducts(outputDirectory string, products []types.ExpectedProduct) error {
	for _, product := range products {
		if err := validateProduct(outputDirectory, product); err != nil {
			return err
		}
	}
	return nil
}

func validateProduct(outputDirectory string, product types.ExpectedProduct) error {
	if product.RelativePath == "" || filepath.IsAbs(product.RelativePath) {
		return fmt.Errorf("invalid expected product path %q", product.RelativePath)
	}
	cleanRelativePath := filepath.Clean(product.RelativePath)
	if cleanRelativePath == ".." || strings.HasPrefix(cleanRelativePath, ".."+string(filepath.Separator)) {
		return fmt.Errorf("expected product path %q escapes the output directory", product.RelativePath)
	}

	productPath := filepath.Join(outputDirectory, cleanRelativePath)
	productInfo, err := os.Lstat(productPath)
	if err != nil {
		return fmt.Errorf("required product %q is unavailable: %w", product.RelativePath, err)
	}
	if !productInfo.Mode().IsRegular() || productInfo.Size() == 0 {
		return fmt.Errorf("required product %q must be a non-empty regular file", product.RelativePath)
	}

	file, err := os.Open(productPath)
	if err != nil {
		return fmt.Errorf("open required product %q: %w", product.RelativePath, err)
	}
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.FieldsPerRecord = -1
	header, readErr := reader.Read()
	closeErr := file.Close()
	if readErr != nil {
		if errors.Is(readErr, io.EOF) {
			return fmt.Errorf("required product %q has no header", product.RelativePath)
		}
		return fmt.Errorf("read required product %q header: %w", product.RelativePath, readErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close required product %q: %w", product.RelativePath, closeErr)
	}

	headerColumns := make(map[string]struct{}, len(header))
	for _, column := range header {
		headerColumns[strings.TrimSpace(column)] = struct{}{}
	}
	for _, requiredColumn := range product.RequiredColumns {
		if _, exists := headerColumns[requiredColumn]; !exists {
			return fmt.Errorf("required product %q is missing column %q", product.RelativePath, requiredColumn)
		}
	}
	return nil
}

func publishDirectory(stagingDirectory, finalDirectory string) error {
	finalInfo, err := os.Lstat(finalDirectory)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.Rename(stagingDirectory, finalDirectory); err != nil {
			return err
		}
		return syncDirectory(filepath.Dir(finalDirectory))
	}
	if err != nil {
		return err
	}
	if !finalInfo.IsDir() || finalInfo.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("final output %q must be a real directory", finalDirectory)
	}

	backupDirectory, err := reserveSiblingPath(finalDirectory, "backup")
	if err != nil {
		return err
	}
	if err := os.Rename(finalDirectory, backupDirectory); err != nil {
		return fmt.Errorf("move previous output to backup: %w", err)
	}
	if err := os.Rename(stagingDirectory, finalDirectory); err != nil {
		rollbackErr := os.Rename(backupDirectory, finalDirectory)
		if rollbackErr != nil {
			return fmt.Errorf("publish new output: %v; rollback previous output: %w", err, rollbackErr)
		}
		return fmt.Errorf("publish new output: %w", err)
	}
	if err := syncDirectory(filepath.Dir(finalDirectory)); err != nil {
		return err
	}
	if err := os.RemoveAll(backupDirectory); err != nil {
		return fmt.Errorf("remove previous output backup %q: %w", backupDirectory, err)
	}
	return syncDirectory(filepath.Dir(finalDirectory))
}

func reserveSiblingPath(finalPath, purpose string) (string, error) {
	parentDirectory := filepath.Dir(finalPath)
	placeholder, err := os.CreateTemp(
		parentDirectory,
		"."+filepath.Base(finalPath)+"."+purpose+"-",
	)
	if err != nil {
		return "", err
	}
	placeholderPath := placeholder.Name()
	if err := placeholder.Close(); err != nil {
		_ = os.Remove(placeholderPath)
		return "", err
	}
	if err := os.Remove(placeholderPath); err != nil {
		return "", err
	}
	return placeholderPath, nil
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	if err := directory.Sync(); err != nil {
		_ = directory.Close()
		return err
	}
	return directory.Close()
}
