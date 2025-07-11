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

func main() {
	var (
		hunks     = flag.String("hunks", "", "Comma-separated list of hunk numbers to stage (e.g., 1,3,5)")
		patchFile = flag.String("patch", "", "Path to the patch file")
	)
	
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s -hunks=<hunk_list> -patch=<patch_file>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nStages specified hunks from a patch file sequentially.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExample:\n")
		fmt.Fprintf(os.Stderr, "  %s -hunks=1,3,5 -patch=changes.patch\n", os.Args[0])
	}
	
	flag.Parse()
	
	// Create real command executor
	exec := executor.NewRealCommandExecutor()
	
	// Check dependencies
	v := validator.NewValidator(exec)
	if err := v.CheckDependencies(); err != nil {
		log.Fatalf("Dependency check failed: %v", err)
	}
	
	// Validate arguments
	if err := v.ValidateArgs(*hunks, *patchFile); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
		flag.Usage()
		os.Exit(1)
	}
	
	// Stage hunks
	s := stager.NewStager(exec)
	if err := s.StageHunks(*hunks, *patchFile); err != nil {
		log.Fatalf("Failed to stage hunks: %v", err)
	}
	
	fmt.Printf("Successfully staged hunks: %s\n", *hunks)
}