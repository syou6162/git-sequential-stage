package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/syou6162/git-sequential-stage/internal/executor"
	"github.com/syou6162/git-sequential-stage/internal/stager"
	"github.com/syou6162/git-sequential-stage/internal/validator"
)

func main() {
	var (
		hunks     = flag.String("hunks", "", "Comma-separated list of hunk numbers to stage (e.g., 1,3,5)")
		patchFile = flag.String("patch", "", "Path to the patch file")
		showHunks = flag.Bool("show-hunks", false, "Show all hunks with their patch IDs (for debugging)")
	)
	
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s -hunks=<hunk_list> -patch=<patch_file>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nStages specified hunks from a patch file sequentially.\n")
		fmt.Fprintf(os.Stderr, "Uses patch IDs internally to ensure reliable hunk identification.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExample:\n")
		fmt.Fprintf(os.Stderr, "  %s -hunks=1,3,5 -patch=changes.patch\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nDebug:\n")
		fmt.Fprintf(os.Stderr, "  %s -show-hunks -patch=changes.patch\n", os.Args[0])
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
	
	// Handle show-hunks mode
	if *showHunks {
		if err := showHunksWithPatchIDs(s, *patchFile); err != nil {
			log.Fatalf("Failed to show hunks: %v", err)
		}
		return
	}
	
	// Check dependencies
	v := validator.NewValidator(exec)
	if err := v.CheckDependencies(); err != nil {
		log.Fatalf("Dependency check failed: %v", err)
	}
	
	// Validate arguments
	if *hunks == "" {
		fmt.Fprintf(os.Stderr, "Error: -hunks flag is required\n\n")
		flag.Usage()
		os.Exit(1)
	}
	
	if err := v.ValidateArgs(*hunks, *patchFile); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
		flag.Usage()
		os.Exit(1)
	}
	
	// Stage hunks
	if err := s.StageHunks(*hunks, *patchFile); err != nil {
		handleStageError(err)
	}
	
	fmt.Printf("Successfully staged hunks: %s\n", *hunks)
}

func handleStageError(err error) {
	fmt.Fprintf(os.Stderr, "Failed to stage hunks: %v\n\n", err)
	fmt.Fprintf(os.Stderr, "Troubleshooting tips:\n")
	fmt.Fprintf(os.Stderr, "1. Check if the patch file exists and is readable\n")
	fmt.Fprintf(os.Stderr, "2. Verify that the hunks haven't already been staged\n")
	fmt.Fprintf(os.Stderr, "3. Ensure the patch was generated from the current working tree state\n")
	fmt.Fprintf(os.Stderr, "4. Run 'git status' to check the current state\n")
	fmt.Fprintf(os.Stderr, "5. Use -show-hunks to see all available hunks and their patch IDs\n")
	os.Exit(1)
}

func showHunksWithPatchIDs(s *stager.Stager, patchFile string) error {
	content, err := os.ReadFile(patchFile)
	if err != nil {
		return fmt.Errorf("failed to read patch file: %v", err)
	}
	
	hunks, err := stager.ExtractHunksFromPatch(string(content))
	if err != nil {
		return fmt.Errorf("failed to parse patch file: %v", err)
	}
	
	fmt.Printf("Found %d hunks in patch file:\n\n", len(hunks))
	
	for _, hunk := range hunks {
		fmt.Printf("Hunk #%d (Patch ID: %s)\n", hunk.Number, hunk.PatchID)
		fmt.Printf("File: %s\n", hunk.FilePath)
		fmt.Printf("Header: %s\n", hunk.Header)
		fmt.Println(strings.Repeat("-", 60))
	}
	
	return nil
}