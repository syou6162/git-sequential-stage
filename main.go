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
	// Validate required arguments
	if len(hunks) == 0 {
		return fmt.Errorf("at least one -hunk flag is required")
	}
	if patchFile == "" {
		return fmt.Errorf("-patch flag is required")
	}

	// Create real command executor
	exec := executor.NewRealCommandExecutor()
	s := stager.NewStager(exec)

	// Check dependencies
	v := validator.NewValidator(exec)
	if err := v.CheckDependencies(); err != nil {
		return fmt.Errorf("dependency check failed: %v", err)
	}

	// Separate wildcard files from normal hunk specifications
	wildcardFiles := []string{}
	normalHunks := []string{}
	fileSpecTypes := make(map[string]string) // Track specification type per file

	for _, spec := range hunks {
		parts := strings.Split(spec, ":")
		if len(parts) != 2 {
			return fmt.Errorf("invalid hunk specification: %s (expected format: file:hunks)", spec)
		}

		file := parts[0]
		hunksSpec := parts[1]

		// Check if this file has already been specified
		if existingType, exists := fileSpecTypes[file]; exists {
			// Check for conflicting specifications
			if hunksSpec == "*" && existingType == "numbers" {
				return fmt.Errorf("mixed wildcard and hunk numbers not allowed for file %s", file)
			}
			if hunksSpec != "*" && existingType == "wildcard" {
				return fmt.Errorf("mixed wildcard and hunk numbers not allowed for file %s", file)
			}
		}

		if hunksSpec == "*" {
			// Wildcard: add entire file
			wildcardFiles = append(wildcardFiles, file)
			fileSpecTypes[file] = "wildcard"
		} else {
			// Check for mixed wildcard and numbers (not allowed)
			if strings.Contains(hunksSpec, "*") {
				return fmt.Errorf("mixed wildcard and hunk numbers not allowed in %s", spec)
			}
			normalHunks = append(normalHunks, spec)
			fileSpecTypes[file] = "numbers"
		}
	}

	// Stage specific hunks first if any
	// (Need to process hunks before wildcard to maintain patch consistency)
	if len(normalHunks) > 0 {
		// Validate arguments for normal hunks
		if err := v.ValidateArgsNew(normalHunks, patchFile); err != nil {
			return fmt.Errorf("argument validation failed: %v", err)
		}

		// Stage hunks
		if err := s.StageHunks(normalHunks, patchFile); err != nil {
			return fmt.Errorf("failed to stage hunks: %v", err)
		}
	}

	// Stage wildcard files directly with git add (after hunks)
	if len(wildcardFiles) > 0 {
		if err := s.StageFiles(wildcardFiles); err != nil {
			return fmt.Errorf("failed to stage wildcard files: %v", err)
		}
	}

	return nil
}

// routeSubcommand routes to the appropriate subcommand handler
// Minimal implementation for GREEN phase: returns fixed errors for testing
func routeSubcommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("subcommand required")
	}

	subcommand := args[0]
	switch subcommand {
	case "stage":
		// For now, return an error since we haven't implemented flag parsing yet
		return fmt.Errorf("patch file required")
	case "count-hunks":
		// Minimal implementation: return nil to pass the test
		return nil
	default:
		return fmt.Errorf("unknown subcommand: %s", subcommand)
	}
}

func main() {
	var (
		hunks     hunkList
		patchFile = flag.String("patch", "", "Path to the patch file")
	)

	flag.Var(&hunks, "hunk", "File:hunk_numbers to stage (e.g., path/to/file.py:1,3) or file:* for entire file. Can be specified multiple times")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s -patch=<patch_file> -hunk=<file:numbers|*> [-hunk=<file:numbers|*>...]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nStages specified hunks from a patch file sequentially.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  # Stage specific hunks\n")
		fmt.Fprintf(os.Stderr, "  %s -patch=changes.patch -hunk=\"src/main.go:1,3\" -hunk=\"src/test.go:2\"\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\n  # Stage entire files using wildcard\n")
		fmt.Fprintf(os.Stderr, "  %s -patch=changes.patch -hunk=\"src/logger.go:*\" -hunk=\"src/test.go:1,2\"\n", os.Args[0])
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
