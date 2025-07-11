package stager

import (
	"bytes"
	"os"
	"os/exec"
	"testing"
)

// TestFilterdiffBehavior demonstrates the actual filterdiff behavior with multi-file patches
func TestFilterdiffBehavior(t *testing.T) {
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

	// Single file patch (just validator.go)
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

	// Create temp files
	multiFile, err := os.CreateTemp("", "multi*.patch")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(multiFile.Name())
	multiFile.WriteString(multiFilePatch)
	multiFile.Close()
	
	singleFile, err := os.CreateTemp("", "single*.patch") 
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(singleFile.Name())
	singleFile.WriteString(singleFilePatch)
	singleFile.Close()
	
	// Extract hunk 1 from multi-file patch
	cmd1 := exec.Command("filterdiff", "--hunks=1", multiFile.Name())
	out1, err := cmd1.Output()
	if err != nil {
		t.Fatalf("filterdiff multi-file failed: %v", err)
	}
	
	// Extract hunk 1 from single-file patch  
	cmd2 := exec.Command("filterdiff", "--hunks=1", singleFile.Name())
	out2, err := cmd2.Output()
	if err != nil {
		t.Fatalf("filterdiff single-file failed: %v", err)
	}
	
	// Compare outputs
	if !bytes.Equal(out1, out2) {
		t.Logf("Multi-file extraction (%d bytes):\n%s", len(out1), out1)
		t.Logf("Single-file extraction (%d bytes):\n%s", len(out2), out2)
		t.Error("filterdiff produces different outputs for the same hunk from multi-file vs single-file patches")
	}
	
	// Check patch IDs
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}
	
	// Get patch ID from multi-file extraction
	cmd3 := exec.Command("git", "patch-id", "--stable")
	cmd3.Stdin = bytes.NewReader(out1)
	patchID1, err := cmd3.Output()
	if err != nil {
		t.Fatalf("git patch-id failed: %v", err)
	}
	
	// Get patch ID from single-file extraction
	cmd4 := exec.Command("git", "patch-id", "--stable")
	cmd4.Stdin = bytes.NewReader(out2) 
	patchID2, err := cmd4.Output()
	if err != nil {
		t.Fatalf("git patch-id failed: %v", err)
	}
	
	if !bytes.Equal(patchID1, patchID2) {
		t.Logf("Multi-file patch ID: %s", patchID1)
		t.Logf("Single-file patch ID: %s", patchID2)
		t.Error("Different patch IDs for the same logical hunk")
	}
}