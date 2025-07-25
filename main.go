package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/syou6162/git-sequential-stage/internal/executor"
	"github.com/syou6162/git-sequential-stage/internal/stager"
	"github.com/syou6162/git-sequential-stage/internal/validator"
)

// Custom type to handle multiple -hunk flags
type hunkList []string

func (h *hunkList) String() string {
	return strings.Join(*h, ", ")
}

func (h *hunkList) Set(value string) error {
	*h = append(*h, value)
	return nil
}

// runGitSequentialStage は git-sequential-stage の主要なロジックを実行します
// テストから直接呼び出せるように分離されています
func runGitSequentialStage(hunks []string, patchFile string) error {
	// Create real command executor
	exec := executor.NewRealCommandExecutor()
	s := stager.NewStager(exec)

	// Check dependencies
	v := validator.NewValidator(exec)
	if err := v.CheckDependencies(); err != nil {
		return fmt.Errorf("dependency check failed: %v", err)
	}

	// Validate arguments
	if err := v.ValidateArgsNew(hunks, patchFile); err != nil {
		return fmt.Errorf("argument validation failed: %v", err)
	}

	// Stage hunks
	if err := s.StageHunks(hunks, patchFile); err != nil {
		return fmt.Errorf("failed to stage hunks: %v", err)
	}

	return nil
}

func main() {
	var (
		hunks     hunkList
		patchFile = flag.String("patch", "", "Path to the patch file")
	)

	flag.Var(&hunks, "hunk", "File:hunk_numbers to stage (e.g., path/to/file.py:1,3). Can be specified multiple times")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s -patch=<patch_file> -hunk=<file:numbers> [-hunk=<file:numbers>...]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nStages specified hunks from a patch file sequentially.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExample:\n")
		fmt.Fprintf(os.Stderr, "  %s -patch=changes.patch -hunk=\"src/main.go:1,3\" -hunk=\"src/test.go:2\"\n", os.Args[0])
	}

	flag.Parse()

	// Validate patch file is provided
	if *patchFile == "" {
		fmt.Fprintf(os.Stderr, "Error: -patch flag is required\n\n")
		flag.Usage()
		os.Exit(1)
	}

	// Validate arguments
	if len(hunks) == 0 {
		fmt.Fprintf(os.Stderr, "Error: at least one -hunk flag is required\n\n")
		flag.Usage()
		os.Exit(1)
	}

	// Run the main logic
	if err := runGitSequentialStage(hunks, *patchFile); err != nil {
		handleStageError(err)
	}

	fmt.Printf("Successfully staged specified hunks\n")
}

func handleStageError(err error) {
	fmt.Fprintf(os.Stderr, "Failed to stage hunks: %v\n\n", err)

	fmt.Fprintf(os.Stderr, "Troubleshooting tips:\n")
	fmt.Fprintf(os.Stderr, "1. Check if the patch file exists and is readable\n")
	fmt.Fprintf(os.Stderr, "2. Verify that the hunks haven't already been staged\n")
	fmt.Fprintf(os.Stderr, "3. Ensure the patch was generated from the current working tree state\n")
	fmt.Fprintf(os.Stderr, "4. Run 'git status' to check the current state\n")
	fmt.Fprintf(os.Stderr, "\nFor detailed debug output, set GIT_SEQUENTIAL_STAGE_VERBOSE=1\n")
	os.Exit(1)
}
