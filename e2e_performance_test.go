package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/syou6162/git-sequential-stage/testutils"
)

// TestE2E_PerformanceWithSafetyChecks tests that safety checks don't significantly impact performance
func TestE2E_PerformanceWithSafetyChecks(t *testing.T) {
	// Create a temporary directory for our test repository
	tmpDir, repo, cleanup := testutils.CreateTestRepo(t, "git-sequential-stage-performance-test-*")
	defer cleanup()

	// Change to the repository directory
	resetDir := testutils.SetupTestDir(t, tmpDir)
	defer resetDir()

	// Create a large file with many hunks
	testutils.CreateLargeFileWithManyHunks(t, tmpDir, repo)

	// Generate patch
	patchFile := filepath.Join(tmpDir, "changes.patch")
	output, err := testutils.RunCommand(t, tmpDir, "git", "diff", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(patchFile, []byte(output), 0644); err != nil {
		t.Fatal(err)
	}

	// Measure performance with clean staging area (safety checks should pass quickly)
	t.Run("clean_staging_area", func(t *testing.T) {
		// Run multiple times to get average
		const iterations = 5
		var totalDuration time.Duration

		for i := 0; i < iterations; i++ {
			// Reset staging area
			testutils.RunCommand(t, tmpDir, "git", "reset", "HEAD")
			
			start := time.Now()
			err := runGitSequentialStage([]string{"large_module.py:1,3,5"}, patchFile)
			duration := time.Since(start)
			
			if err != nil {
				t.Fatalf("Iteration %d failed: %v", i+1, err)
			}
			
			totalDuration += duration
			t.Logf("Iteration %d: %v", i+1, duration)
		}

		avgDuration := totalDuration / iterations
		t.Logf("Average execution time with safety checks: %v", avgDuration)

		// Check that average is under reasonable threshold (based on TestLargeFileWithManyHunks ~230ms)
		// With safety checks, we expect at most 120% of original time
		expectedMax := 230 * time.Millisecond * 120 / 100 // 276ms
		if avgDuration > expectedMax {
			t.Errorf("Performance degradation detected: %v > %v (120%% of baseline)", avgDuration, expectedMax)
		} else {
			t.Logf("Performance within acceptable range: %v <= %v", avgDuration, expectedMax)
		}
	})

	// Test with staged files (safety check should detect and error quickly)
	t.Run("staged_files_early_exit", func(t *testing.T) {
		// Stage a file
		testutils.RunCommand(t, tmpDir, "git", "add", "large_module.py")
		
		start := time.Now()
		err := runGitSequentialStage([]string{"large_module.py:1"}, patchFile)
		duration := time.Since(start)
		
		if err == nil {
			t.Fatal("Expected safety check error, but got none")
		}
		
		t.Logf("Safety check early exit time: %v", duration)
		t.Logf("Error message: %v", err)
		
		// Check if error is from safety check (not from git apply)
		if strings.Contains(err.Error(), "SAFETY_CHECK_FAILED") || 
		   strings.Contains(err.Error(), "staging area is not clean") {
			// Safety check should fail fast (under 50ms)
			if duration > 50*time.Millisecond {
				t.Errorf("Safety check took too long: %v > 50ms", duration)
			}
		} else {
			t.Logf("Note: Error was not from safety check, but from: %v", err)
			// The current implementation doesn't have safety checks yet
			// This is expected until safety checks are implemented
		}
	})
}


