package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/syou6162/git-sequential-stage/internal/executor"
	"github.com/syou6162/git-sequential-stage/internal/stager"
	"github.com/syou6162/git-sequential-stage/internal/validator"
)

// Custom type to handle multiple -hunk flags
type hunkList []string

func (h *hunkList) String() string {
	return ""
}

func (h *hunkList) Set(value string) error {
	*h = append(*h, value)
	return nil
}

func main() {
	var (
		hunks     hunkList
		hunksStr  = flag.String("hunks", "", "Comma-separated list of hunk numbers to stage (e.g., 1,3,5)")
		patchFile = flag.String("patch", "", "Path to the patch file")
	)
	
	flag.Var(&hunks, "hunk", "File:hunk_numbers to stage (e.g., path/to/file.py:1,3). Can be specified multiple times")
	
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s -patch=<patch_file> {-hunks=<numbers> | -hunk=<file:numbers> [-hunk=<file:numbers>...]}\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nStages specified hunks from a patch file sequentially.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s -patch=changes.patch -hunks=\"1,3,5\"\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -patch=changes.patch -hunk=\"src/main.go:1,3\" -hunk=\"src/test.go:2\"\n", os.Args[0])
	}
	
	flag.Parse()
	
	// Validate patch file is provided
	if *patchFile == "" {
		fmt.Fprintf(os.Stderr, "Error: -patch flag is required\n\n")
		flag.Usage()
		os.Exit(1)
	}
	
	// Create real command executor
	exec := executor.NewRealCommandExecutor()
	s := stager.NewStager(exec)
	
	// Check dependencies
	v := validator.NewValidator(exec)
	if err := v.CheckDependencies(); err != nil {
		log.Fatalf("Dependency check failed: %v", err)
	}
	
	// Check for mutually exclusive flags
	if *hunksStr != "" && len(hunks) > 0 {
		fmt.Fprintf(os.Stderr, "Error: -hunks and -hunk flags are mutually exclusive\n\n")
		flag.Usage()
		os.Exit(1)
	}
	
	// Validate arguments
	if *hunksStr == "" && len(hunks) == 0 {
		fmt.Fprintf(os.Stderr, "Error: either -hunks or -hunk flag is required\n\n")
		flag.Usage()
		os.Exit(1)
	}
	
	// Stage hunks based on which flag was used
	if *hunksStr != "" {
		// Use simple hunk numbers format
		if err := v.ValidateArgs(*hunksStr, *patchFile); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
			flag.Usage()
			os.Exit(1)
		}
		
		if err := s.StageHunks(*hunksStr, *patchFile); err != nil {
			handleStageError(err)
		}
	} else {
		// Use file:hunk format
		if err := v.ValidateArgsNew(hunks, *patchFile); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
			flag.Usage()
			os.Exit(1)
		}
		
		if err := s.StageHunksNew(hunks, *patchFile); err != nil {
			handleStageError(err)
		}
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
	os.Exit(1)
}