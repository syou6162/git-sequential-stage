# git-sequential-stage

A Go CLI tool that stages specified hunks from a patch file sequentially.

## Background and Motivation

This tool was developed to solve challenges faced when using LLM agents to create semantically meaningful commits from code changes.

### Problems to Solve

When having LLM agents create commits, we encountered the following issues:

1. Difficulty in creating semantically cohesive commits: It's challenging to select only semantically related parts from changes across multiple files
2. Hunk dependencies: When certain hunks depend on others, simple `git add -p` cannot handle the ordering
3. Need for automation: LLM agents need a programmatic way to select and stage hunks

### Why This Tool is Needed

- LLM-friendly: Programmatically controllable by simply specifying hunk numbers via command-line arguments
- Sequential application: Applies hunks in the specified order, correctly handling dependent hunks
- Error handling: Passes through `git apply` errors directly, making it easy for LLMs to understand issues

This tool enables LLM agents to create "semantically cohesive and clean commits" just like humans do.

## Purpose

This tool solves the problem of selectively staging multiple hunks from a patch file one by one, ensuring each hunk is applied in sequence. It's particularly useful when you need fine-grained control over which changes to stage.

## Prerequisites

- `git` command must be installed
- `filterdiff` command must be installed (part of the `patchutils` package)

## Installation

```bash
go install github.com/syou6162/git-sequential-stage@latest
```

Or build from source:

```bash
git clone https://github.com/syou6162/git-sequential-stage.git
cd git-sequential-stage
go build
```

## Usage

```bash
# Basic usage with hunk numbers
git-sequential-stage -hunks=<hunk_list> -patch=<patch_file>

# Advanced usage with patch IDs
git-sequential-stage -patch-ids=<patch_id_list> -patch=<patch_file>

# Show all hunks with their patch IDs
git-sequential-stage -show-hunks -patch=<patch_file>
```

### Options

- `-hunks`: Comma-separated list of hunk numbers to stage (e.g., "1,3,5")
- `-patch-ids`: Comma-separated list of patch IDs to stage (more reliable for complex workflows)
- `-patch`: Path to the patch file
- `-show-hunks`: Display all hunks with their patch IDs for inspection

### Examples

```bash
# Generate a patch file
git diff > changes.patch

# Traditional: Stage by hunk numbers
git-sequential-stage -hunks=1,3,5 -patch=changes.patch

# Advanced: First, inspect hunks and their patch IDs
git-sequential-stage -show-hunks -patch=changes.patch

# Then stage by patch IDs (more reliable)
git-sequential-stage -patch-ids=a1b2c3d4,e5f6g7h8 -patch=changes.patch
```

### Patch ID Mode

The patch ID mode is designed for integration with LLM agents and semantic commit workflows:

- Each hunk gets a unique 8-character patch ID based on its content
- Patch IDs remain stable even if hunk numbers change due to partial staging
- Perfect for workflows where an LLM analyzes patches and selects hunks semantically

## How it works

1. Validates that `git` and `filterdiff` commands are available
2. Parses the hunk numbers from the command line
3. For each hunk number:
   - Extracts the hunk using `filterdiff --hunks=N`
   - Applies it to the staging area using `git apply --cached`
4. If any hunk fails to apply, the tool stops and reports the error

## Development

### Running tests

```bash
go test ./...
```

### Project structure

```
git-sequential-stage/
├── main.go                 # CLI entry point
├── internal/
│   ├── executor/          # Command execution abstraction
│   ├── stager/            # Core staging logic
│   └── validator/         # Dependency and argument validation
```

## License

MIT License - see LICENSE file for details