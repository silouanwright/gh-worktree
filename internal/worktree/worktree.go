package worktree

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cli/safeexec"
)

func Add(branch string, path string) error {
	return AddWithOptions(branch, path, false)
}

func AddWithOptions(branch string, path string, appendBranch bool) error {
	var branchPath string
	if path != "" {
		if appendBranch {
			branchPath = filepath.Join(path, branch)
		} else {
			branchPath = path
		}
	} else {
		gitPath, err := getCommonGitDirectory()
		if err != nil {
			return fmt.Errorf("could not get working directory: %w", err)
		}

		branchPath = filepath.Join(gitPath, branch)
	}

	// Check if worktree already exists for this branch
	existingPath, err := getWorktreePathForBranch(branch)
	if err == nil && existingPath != "" {
		return fmt.Errorf("worktree for branch '%s' already exists at: %s", branch, existingPath)
	}

	// Check if the target directory already exists
	if _, err := os.Stat(branchPath); err == nil {
		return fmt.Errorf("directory already exists at: %s\nPlease remove it or choose a different path", branchPath)
	}

	cmdArgs := []string{"worktree", "add", branchPath, branch}

	output, err := git(cmdArgs)
	if err != nil {
		// Parse git error for better messaging
		if strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("worktree or branch '%s' already exists\nUse 'git worktree list' to see existing worktrees", branch)
		}
		if strings.Contains(err.Error(), "invalid reference") {
			return fmt.Errorf("branch '%s' not found\nMake sure the branch exists or the PR has been fetched", branch)
		}
		return fmt.Errorf("failed to create worktree: %w\nOutput: %s", err, string(output))
	}
	return nil
}

func getWorktreePathForBranch(branch string) (string, error) {
	args := []string{"worktree", "list", "--porcelain"}
	output, err := git(args)
	if err != nil {
		return "", err
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var currentPath string
	for _, line := range lines {
		if strings.HasPrefix(line, "worktree ") {
			currentPath = strings.TrimPrefix(line, "worktree ")
		}
		// Check both local branches and detached heads that might match the branch name
		if strings.HasPrefix(line, "branch refs/heads/") {
			currentBranch := strings.TrimPrefix(line, "branch refs/heads/")
			if currentBranch == branch {
				return currentPath, nil
			}
		}
		// Also check if the path ends with the branch name (common pattern)
		if currentPath != "" && filepath.Base(currentPath) == branch {
			return currentPath, nil
		}
	}
	return "", fmt.Errorf("worktree for branch %s not found", branch)
}

func getCommonGitDirectory() (string, error) {
	args := []string{"rev-parse", "--git-common-dir"}
	b, err := git(args)
	if err != nil {
		return "", fmt.Errorf("could not get git common dir: %w", err)
	}

	root := filepath.Join(string(b), "..")

	return root, nil
}

func git(args []string) ([]byte, error) {
	cmd, err := safeexec.LookPath("git")
	if err != nil {
		return nil, err
	}
	c := exec.Command(cmd, args...)

	return c.Output()
}
