package cli

import (
	"path/filepath"
	"testing"
)

func TestContrastDirectoriesIncludeSpecies(t *testing.T) {
	rootDir := t.TempDir()
	outputDir, tempDir, err := contrastDirectories(rootDir, "human", "contrast-001")
	if err != nil {
		t.Fatalf("contrastDirectories returned error: %v", err)
	}

	expectedOutputDir := filepath.Join(rootDir, "RNASplicing", "human", "contrast-001")
	if outputDir != expectedOutputDir {
		t.Fatalf("output directory = %q, want %q", outputDir, expectedOutputDir)
	}

	expectedTempDir := filepath.Join(expectedOutputDir, "temp")
	if tempDir != expectedTempDir {
		t.Fatalf("temp directory = %q, want %q", tempDir, expectedTempDir)
	}
}

func TestContrastDirectoriesRejectPathTraversal(t *testing.T) {
	testCases := []struct {
		name    string
		species string
		taskID  string
	}{
		{name: "parent species", species: "..", taskID: "contrast-001"},
		{name: "absolute species", species: "/tmp/escape", taskID: "contrast-001"},
		{name: "nested task", species: "human", taskID: "../../escape"},
		{name: "windows separator", species: `human\\escape`, taskID: "contrast-001"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if _, _, err := contrastDirectories(t.TempDir(), testCase.species, testCase.taskID); err == nil {
				t.Fatalf("expected path validation error for species %q task %q", testCase.species, testCase.taskID)
			}
		})
	}
}
