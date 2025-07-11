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
git-sequential-stage -hunks=<hunk_list> -patch=<patch_file>
```

### Options

- `-hunks`: Comma-separated list of hunk numbers to stage (e.g., "1,3,5")
- `-patch`: Path to the patch file

### Example

```bash
# Generate a patch file
git diff > changes.patch

# Stage hunks 1, 3, and 5 from the patch
git-sequential-stage -hunks=1,3,5 -patch=changes.patch
```

### How It Works Internally

The tool uses patch IDs internally to ensure reliable hunk identification:

1. When you specify hunk numbers (e.g., 1,3,5), the tool:
   - Parses the entire patch file
   - Assigns a unique patch ID to each hunk based on its content
   - Uses these IDs internally to track and apply hunks
   
2. This approach solves the "hunk number drift" problem:
   - Even if applying hunk 1 would change line numbers for subsequent hunks
   - The tool can still correctly identify and apply hunks 3 and 5
   - Because it uses content-based IDs, not line numbers

This makes the tool perfect for LLM agent workflows where semantic commit splitting is required.

## How it works

1. Validates that `git` command is available
2. Parses the hunk numbers from the command line
3. Reads and parses the patch file to extract all hunks
4. For each requested hunk number:
   - Finds the hunk by its number
   - Generates a unique patch ID based on content
   - Applies it to the staging area using `git apply --cached`
5. If any hunk fails to apply, the tool stops and reports the error with detailed information

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