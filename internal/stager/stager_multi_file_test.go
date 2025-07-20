package stager

import (
	"testing"
)

// This test file documents the issue with multi-file patches
// The actual implementation test is in filterdiff_test.go

func TestStageHunks_MultiFilePatches_Documentation(t *testing.T) {
	// This test documents the expected behavior for multi-file patches
	// Currently, there is an issue where patch IDs differ between:
	// 1. Extracting a hunk from a multi-file patch
	// 2. Extracting the same hunk from a single-file diff

	t.Log("Multi-file patch issue:")
	t.Log("1. User provides a multi-file patch (e.g., git diff HEAD > all.patch)")
	t.Log("2. StageHunks extracts hunk N from the multi-file patch using filterdiff")
	t.Log("3. In execution phase, it gets a fresh diff for the specific file")
	t.Log("4. The patch IDs don't match because filterdiff behaves differently")
	t.Log("")
	t.Log("Current workaround: Use single-file patches")
	t.Log("Potential fix: Ensure consistent patch ID calculation")
}
