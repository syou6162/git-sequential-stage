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

## Intended Use Cases

This tool is primarily designed for integration with LLM agents (such as Claude Code) to automatically split large, complex changes into semantically meaningful commits. While it can be used standalone, its true value emerges when combined with AI-powered development workflows.

**Primary scenarios:**
- LLM agents making large changes that need to be broken down
- Automated semantic commit creation from complex diffs
- Programmatic control over staging for AI development workflows
- Selectively staging multiple hunks from a patch file one by one, ensuring each hunk is applied in sequence
- Fine-grained control over which changes to stage

## AI-Powered Workflow

When integrated with LLM agents, the typical workflow becomes:

1. **Analysis**: LLM analyzes the current diff and identifies semantic units
2. **Planning**: LLM determines optimal commit structure 
3. **Staging**: `git-sequential-stage` applies changes incrementally using patch IDs
4. **Committing**: Each semantic unit becomes a focused, meaningful commit
5. **Iteration**: Process repeats until all changes are committed

This approach ensures that even large, complex changes result in clean, reviewable commit history.

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
git-sequential-stage -patch=<patch_file> -hunk=<file:hunks> [-hunk=<file:hunks>...]
```

### Options

- `-patch`: Path to the patch file
- `-hunk`: File and hunk specification in the format `file:hunk_numbers` (can be specified multiple times)

### Examples

```bash
# Generate a patch file
git diff > changes.patch

# Stage hunks 1 and 3 from main.go
git-sequential-stage -patch=changes.patch -hunk="main.go:1,3"

# Stage multiple files with different hunks
git-sequential-stage -patch=changes.patch \
  -hunk="main.go:1,3" \
  -hunk="internal/stager/stager.go:2,4,5" \
  -hunk="README.md:1"

# Stage all changes from a specific file
git diff main.go > main.patch
git-sequential-stage -patch=main.patch -hunk="main.go:1,2,3"
```

## New Features

### Enhanced Patch Parsing with go-gitdiff

The tool now uses the `go-gitdiff` library for more robust patch parsing, providing:
- Better handling of special file operations (renames, deletions, binary files)
- Fallback to legacy parser for maximum compatibility
- Improved error messages with custom error types

### Improved Architecture

- **Refactored StageHunks**: Split into smaller, focused functions for better maintainability
- **Custom Error Types**: Structured error handling with context information
- **Better Test Coverage**: Enhanced test suite covering edge cases

## How It Works

The tool uses patch IDs internally to ensure reliable hunk identification and sequential staging:

### Internal Process

1. **Validation**: Checks that `git` and `filterdiff` commands are available
2. **Parsing**: Parses the hunk numbers from the command line
3. **Patch ID Assignment**: When you specify hunk numbers (e.g., 1,3,5), the tool:
   - Parses the entire patch file
   - Assigns a unique patch ID to each hunk based on its content
   - Uses these IDs internally to track and apply hunks
4. **Sequential Staging**: For each requested hunk number:
   - Extracts the single hunk using `filterdiff --hunks=N`
   - Calculates its patch ID using `git patch-id`
   - Applies it to the staging area using `git apply --cached`
5. **Error Handling**: If any hunk fails to apply, the tool stops and reports the error with detailed information

### Solving the "Hunk Number Drift" Problem

This approach solves the common issue where applying early hunks changes line numbers for subsequent hunks:
- Even if applying hunk 1 would change line numbers for subsequent hunks
- The tool can still correctly identify and apply hunks 3 and 5
- Because it uses content-based IDs, not line numbers

This makes the tool perfect for LLM agent workflows where semantic commit splitting is required.

## Integration with Claude Code

This tool works seamlessly with Claude Code custom slash commands, particularly the `semantic_commit` command that provides automated semantic commit workflows.

### Installation via Claude Code Custom Slash Commands (CCCSC)

First, install [CCCSC (Claude Code Custom Slash Commands)](https://github.com/hiragram/cccsc), then add the semantic_commit command:

```bash
# Install the semantic_commit command
npx cccsc add syou6162/claude-code-commands/semantic_commit

# Usage in Claude Code
/cccsc:syou6162:claude-code-commands:semantic_commit
```

### Recommended Claude Code Configuration

To enforce semantic commit workflows, add this to your `settings.json`:

```json
{
  "permissions": {
    "deny": [
      "Bash(git add:*)",
      "Bash(git commit -am:*)",
      "Bash(git commit --all:*)"
    ],
    "allow": [
      "Bash(git add -N:*)",
      "Bash(git diff:*)",
      "Bash(git apply:*)"
    ]
  }
}
```

This prevents manual `git add` commands while allowing necessary git operations for the semantic commit workflow.

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

## References

- Related Tools
  - [CCCSC (Claude Code Custom Slash Commands)](https://github.com/hiragram/cccsc) - Framework for creating custom Claude Code slash commands
  - [semantic_commit Claude Code command](https://github.com/syou6162/claude-code-commands/blob/main/semantic_commit.md) - Custom slash command for automated semantic commits
- Documentation
  - [LLMエージェントに意味のあるコミットを強制させる](https://www.yasuhisay.info/entry/2025/07/12/131421) - Detailed workflow explanation and use cases

## License

MIT License - see LICENSE file for details