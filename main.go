package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
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

// usageShownError is an error type that indicates usage was already shown
type usageShownError struct {
	message string
}

func (e *usageShownError) Error() string {
	return e.message
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
	v := validator.NewValidator(exec)

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

// showUsage displays the top-level usage information
func showUsage() {
	fmt.Fprintf(os.Stderr, "Usage: %s <subcommand> [options]\n\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "Subcommands:\n")
	fmt.Fprintf(os.Stderr, "  stage         Stage specified hunks from a patch file\n")
	fmt.Fprintf(os.Stderr, "  count-hunks   Count hunks per file in the current repository\n")
	fmt.Fprintf(os.Stderr, "\nRun '%s <subcommand> --help' for subcommand-specific options.\n", os.Args[0])
}

// runStageCommand handles the 'stage' subcommand
func runStageCommand(args []string) error {
	// Create a new FlagSet for the stage subcommand
	stageFlags := flag.NewFlagSet("stage", flag.ExitOnError)
	var hunks hunkList
	patchFile := stageFlags.String("patch", "", "Path to the patch file")
	stageFlags.Var(&hunks, "hunk", "File:hunk_numbers to stage (e.g., path/to/file.py:1,3) or file:* for entire file")

	stageFlags.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s stage -patch=<patch_file> -hunk=<file:numbers|*> [-hunk=<file:numbers|*>...]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nStages specified hunks from a patch file sequentially.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		stageFlags.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  # Stage specific hunks\n")
		fmt.Fprintf(os.Stderr, "  %s stage -patch=changes.patch -hunk=\"src/main.go:1,3\" -hunk=\"src/test.go:2\"\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # Stage entire files using wildcard\n")
		fmt.Fprintf(os.Stderr, "  %s stage -patch=changes.patch -hunk=\"src/logger.go:*\" -hunk=\"src/test.go:1,2\"\n", os.Args[0])
	}

	if err := stageFlags.Parse(args); err != nil {
		return err
	}

	// Validate required flags
	if *patchFile == "" {
		stageFlags.Usage()
		fmt.Fprintf(os.Stderr, "\nError: patch file required\n")
		return &usageShownError{message: "patch file required"}
	}
	if len(hunks) == 0 {
		stageFlags.Usage()
		fmt.Fprintf(os.Stderr, "\nError: at least one -hunk flag is required\n")
		return &usageShownError{message: "at least one -hunk flag is required"}
	}

	// Call the existing implementation
	if err := runGitSequentialStage(hunks, *patchFile); err != nil {
		handleStageError(err)
		// handleStageError calls os.Exit(1) and never returns
	}

	// Success: display success message
	fmt.Printf("Successfully staged specified hunks\n")
	return nil
}

// runCountHunksCommand handles the 'count-hunks' subcommand
func runCountHunksCommand(args []string) error {
	// Create a new FlagSet for the count-hunks subcommand
	countFlags := flag.NewFlagSet("count-hunks", flag.ExitOnError)

	countFlags.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s count-hunks\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nCount hunks per file in the current repository.\n\n")
		fmt.Fprintf(os.Stderr, "This command runs 'git diff HEAD' and counts the number of hunks for each modified file.\n")
		fmt.Fprintf(os.Stderr, "Output format: <filepath>: <count>\n")
		fmt.Fprintf(os.Stderr, "Files are sorted alphabetically.\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  %s count-hunks\n", os.Args[0])
	}

	if err := countFlags.Parse(args); err != nil {
		return err
	}

	// Create command executor
	exec := executor.NewRealCommandExecutor()

	// Execute git diff HEAD
	output, err := exec.Execute("git", "diff", "HEAD")
	if err != nil {
		return executor.WrapGitError(err, "git diff")
	}

	// Count hunks in diff output
	hunkCounts, err := stager.CountHunksInDiff(string(output))
	if err != nil {
		return fmt.Errorf("failed to count hunks: %w", err)
	}

	// Sort filenames alphabetically
	var filenames []string
	for filename := range hunkCounts {
		filenames = append(filenames, filename)
	}
	sort.Strings(filenames)

	// Output in "filename: count" format
	// For binary files, this will show "*" instead of a number
	for _, filename := range filenames {
		fmt.Printf("%s: %s\n", filename, hunkCounts[filename])
	}

	return nil
}

// routeSubcommand routes to the appropriate subcommand handler
func routeSubcommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("subcommand required")
	}

	subcommand := args[0]
	subcommandArgs := args[1:]

	switch subcommand {
	case "stage":
		return runStageCommand(subcommandArgs)
	case "count-hunks":
		return runCountHunksCommand(subcommandArgs)
	default:
		return fmt.Errorf("unknown subcommand: %s", subcommand)
	}
}

func main() {
	// Check if a subcommand is provided
	if len(os.Args) < 2 {
		showUsage()
		os.Exit(1)
	}

	// Handle global help flag
	if os.Args[1] == "-h" || os.Args[1] == "--help" {
		showUsage()
		os.Exit(0)
	}

	// Check dependencies early (git installation and repository)
	exec := executor.NewRealCommandExecutor()
	v := validator.NewValidator(exec)
	if err := v.CheckDependencies(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Route to subcommand
	if err := routeSubcommand(os.Args[1:]); err != nil {
		// Check if usage was already shown (e.g., by a subcommand)
		if _, ok := err.(*usageShownError); !ok {
			// Usage not shown yet, show top-level usage
			fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
			showUsage()
		}
		os.Exit(1)
	}
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
