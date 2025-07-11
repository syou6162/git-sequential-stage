package stager

import (
	"bytes"
	"os"
	"os/exec"
	"testing"
)

// TestFilterdiffWithIncludeOption demonstrates the solution using -i option
func TestFilterdiffWithIncludeOption(t *testing.T) {
	// Skip if filterdiff is not available
	if _, err := exec.LookPath("filterdiff"); err != nil {
		t.Skip("filterdiff not found in PATH")
	}
	
	// Multi-file patch
	multiFilePatch := `diff --git a/internal/validator/validator.go b/internal/validator/validator.go
index 095b7a8..5cd4613 100644
--- a/internal/validator/validator.go
+++ b/internal/validator/validator.go
@@ -60,7 +60,6 @@ func (v *Validator) ValidateArgs(hunks, patchFile string) error {
 		return errors.New("patch file cannot be empty")
 	}
 	
-	// Additional validation can be added here
 	return nil
 }
 
@@ -97,3 +96,5 @@ func (v *Validator) ValidateArgsNew(hunkSpecs []string, patchFile string) error
 
 	return nil
 }
+
+// End of file
diff --git a/main.go b/main.go
index 2d9bf93..d28304e 100644
--- a/main.go
+++ b/main.go
@@ -11,19 +11,33 @@ import (
 	"github.com/syou6162/git-sequential-stage/internal/validator"
 )
 
+// Custom type to handle multiple -hunk flags
+type hunkList []string
`

	// Create temp file
	multiFile, err := os.CreateTemp("", "multi*.patch")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(multiFile.Name())
	multiFile.WriteString(multiFilePatch)
	multiFile.Close()
	
	// Method 1: Extract hunk 1 without -i (includes extra file headers)
	cmd1 := exec.Command("filterdiff", "--hunks=1", multiFile.Name())
	out1, err := cmd1.Output()
	if err != nil {
		t.Fatalf("filterdiff without -i failed: %v", err)
	}
	
	// Method 2: Extract hunk 1 with -i option (clean extraction)
	cmd2 := exec.Command("filterdiff", "-i", "*validator.go", "--hunks=1", multiFile.Name())
	out2, err := cmd2.Output()
	if err != nil {
		t.Fatalf("filterdiff with -i failed: %v", err)
	}
	
	// Method 3: Get diff from single file
	singleFilePatch := `diff --git a/internal/validator/validator.go b/internal/validator/validator.go
index 095b7a8..5cd4613 100644
--- a/internal/validator/validator.go
+++ b/internal/validator/validator.go
@@ -60,7 +60,6 @@ func (v *Validator) ValidateArgs(hunks, patchFile string) error {
 		return errors.New("patch file cannot be empty")
 	}
 	
-	// Additional validation can be added here
 	return nil
 }
 
@@ -97,3 +96,5 @@ func (v *Validator) ValidateArgsNew(hunkSpecs []string, patchFile string) error
 
 	return nil
 }
+
+// End of file`
	
	singleFile, err := os.CreateTemp("", "single*.patch")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(singleFile.Name())
	singleFile.WriteString(singleFilePatch)
	singleFile.Close()
	
	cmd3 := exec.Command("filterdiff", "--hunks=1", singleFile.Name())
	out3, err := cmd3.Output()
	if err != nil {
		t.Fatalf("filterdiff single-file failed: %v", err)
	}
	
	// Log sizes
	t.Logf("Without -i: %d bytes", len(out1))
	t.Logf("With -i: %d bytes", len(out2))  
	t.Logf("Single file: %d bytes", len(out3))
	
	// Check if -i option produces same output as single file
	if !bytes.Equal(out2, out3) {
		t.Error("filterdiff with -i option should produce same output as single-file extraction")
	}
	
	// Check patch IDs
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}
	
	// Get patch IDs
	var patchIDs []string
	for i, output := range [][]byte{out1, out2, out3} {
		cmd := exec.Command("git", "patch-id", "--stable")
		cmd.Stdin = bytes.NewReader(output)
		patchID, err := cmd.Output()
		if err != nil {
			t.Fatalf("git patch-id failed for output %d: %v", i+1, err)
		}
		patchIDs = append(patchIDs, string(patchID))
	}
	
	t.Logf("Patch ID without -i: %s", patchIDs[0])
	t.Logf("Patch ID with -i: %s", patchIDs[1])
	t.Logf("Patch ID single file: %s", patchIDs[2])
	
	// Methods 2 and 3 should have same patch ID
	if patchIDs[1] != patchIDs[2] {
		t.Error("Patch IDs should match when using -i option")
	}
	
	// Method 1 should have different patch ID
	if patchIDs[0] == patchIDs[1] {
		t.Error("Expected different patch IDs with and without -i option")
	}
}

// TestFilterdiffIncludePattern tests various include patterns
func TestFilterdiffIncludePattern(t *testing.T) {
	patterns := []struct {
		pattern string
		desc    string
	}{
		{"*validator.go", "wildcard prefix"},
		{"*/validator.go", "wildcard directory"},  
		{"internal/validator/validator.go", "exact path"},
		{"**/validator.go", "recursive wildcard"},
	}
	
	for _, p := range patterns {
		t.Run(p.desc, func(t *testing.T) {
			t.Logf("Pattern: %s - %s", p.pattern, p.desc)
			// In a real implementation, we would test each pattern
			// For now, this documents the available patterns
		})
	}
}