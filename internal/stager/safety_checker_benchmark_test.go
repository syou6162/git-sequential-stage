package stager

import (
	"strings"
	"testing"
	"time"
)

// BenchmarkSafetyChecker_EvaluatePatchContent tests the performance of patch content evaluation
func BenchmarkSafetyChecker_EvaluatePatchContent(b *testing.B) {
	testCases := []struct {
		name  string
		patch string
	}{
		{
			name:  "empty_patch",
			patch: "",
		},
		{
			name: "single_file_modification",
			patch: `diff --git a/file.txt b/file.txt
index 123..456 100644
--- a/file.txt
+++ b/file.txt
@@ -1,3 +1,3 @@
 line1
-old line
+new line
 line3`,
		},
		{
			name: "multiple_files",
			patch: `diff --git a/file1.txt b/file1.txt
index 123..456 100644
--- a/file1.txt
+++ b/file1.txt
@@ -1,3 +1,3 @@
 line1
-old line
+new line
 line3
diff --git a/file2.txt b/file2.txt
index 789..012 100644
--- a/file2.txt
+++ b/file2.txt
@@ -1,3 +1,3 @@
 line1
-old line
+new line
 line3`,
		},
		{
			name: "large_patch_100_files",
			patch: generateLargePatch(100),
		},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			checker := NewSafetyChecker("")
			
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := checker.EvaluatePatchContent(tc.patch)
				if err != nil {
					b.Fatalf("Unexpected error: %v", err)
				}
			}
		})
	}
}

// TestSafetyChecker_PerformanceRequirements tests that safety checks meet performance requirements
func TestSafetyChecker_PerformanceRequirements(t *testing.T) {
	testCases := []struct {
		name        string
		patch       string
		maxDuration time.Duration
	}{
		{
			name:        "empty_patch_under_100ms",
			patch:       "",
			maxDuration: 100 * time.Millisecond,
		},
		{
			name: "small_patch_under_100ms",
			patch: `diff --git a/file.txt b/file.txt
index 123..456 100644
--- a/file.txt
+++ b/file.txt
@@ -1,3 +1,3 @@
 line1
-old line
+new line
 line3`,
			maxDuration: 100 * time.Millisecond,
		},
		{
			name:        "large_patch_100_files_under_100ms",
			patch:       generateLargePatch(100),
			maxDuration: 100 * time.Millisecond,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			checker := NewSafetyChecker("")
			
			start := time.Now()
			_, err := checker.EvaluatePatchContent(tc.patch)
			duration := time.Since(start)
			
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			
			if duration > tc.maxDuration {
				t.Errorf("Performance requirement not met: took %v, expected under %v", duration, tc.maxDuration)
			} else {
				t.Logf("Performance OK: took %v (under %v)", duration, tc.maxDuration)
			}
		})
	}
}

// generateLargePatch generates a patch with the specified number of files
func generateLargePatch(numFiles int) string {
	var builder strings.Builder
	
	for i := 0; i < numFiles; i++ {
		builder.WriteString(generateFilePatch(i))
		if i < numFiles-1 {
			builder.WriteString("\n")
		}
	}
	
	return builder.String()
}

// generateFilePatch generates a patch for a single file
func generateFilePatch(index int) string {
	return strings.ReplaceAll(`diff --git a/file{INDEX}.txt b/file{INDEX}.txt
index 123..456 100644
--- a/file{INDEX}.txt
+++ b/file{INDEX}.txt
@@ -1,3 +1,3 @@
 line1
-old line in file {INDEX}
+new line in file {INDEX}
 line3`, "{INDEX}", string(rune('0'+index)))
}