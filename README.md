# git-sequential-stage

A Go CLI tool that stages specified hunks from a patch file sequentially.

## Purpose

This tool solves the problem of selectively staging multiple hunks from a patch file one by one, ensuring each hunk is applied in sequence. It's particularly useful when you need fine-grained control over which changes to stage.

## Prerequisites

- `git` command must be installed
- `filterdiff` command must be installed (part of the `patchutils` package)

## Installation

```bash
go install github.com/yasuhisa-yoshida/git-sequential-stage@latest
```

Or build from source:

```bash
git clone https://github.com/yasuhisa-yoshida/git-sequential-stage.git
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