# gh-worktree
A github cli extension with an opinionated way of working with git worktree related tasks.

# Installation
```
gh extension install eikster-dk/gh-worktree
```

# usage
```
Work seamlessly across git worktree and gh cli tooling

Usage:
  worktree [command]

Examples:
gh worktree

Available Commands:
  clean       Clean up worktrees for merged/closed PRs and identify stale worktrees
  clone       Will clone a github repository into a folder
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  pr          Will checkout the pr into a worktree branch

Flags:
  -h, --help   help for worktree

Use "worktree [command] --help" for more information about a command.
```

## Commands

### `gh worktree clean`
Automatically removes worktrees for merged or closed PRs. Lists stale worktrees (no commits in 30+ days) for manual review.

```bash
# Clean up merged/closed PR worktrees and review stale ones
gh worktree clean

# Preview what would be cleaned without removing
gh worktree clean --dry-run

# Set custom stale threshold (default: 30 days)
gh worktree clean --stale-days 60
```

### `gh worktree pr`
Checkout a PR into a worktree branch.

```bash
# Checkout PR #123
gh worktree pr 123

# Checkout to specific path
gh worktree pr 123 /path/to/worktree

# Append branch name to path
gh worktree pr 123 /path/to --append-branch
```

### `gh worktree clone`
Clone a repository optimized for worktree usage.

```bash
# Clone a repository
gh worktree clone owner/repo

# Clone to specific directory
gh worktree clone owner/repo my-dir
```
