package worktree

import (
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/cli/safeexec"
)

// Add maintains backwards compatibility - delegates to AddWithOptions with appendBranch=false
func Add(branch string, path string) error {
	return AddWithOptions(branch, path, false)
}

// AddWithOptions allows control over whether to append branch name to the path
func AddWithOptions(branch string, path string, appendBranch bool) error {
	var branchPath string
	if path != "" {
		if appendBranch {
			// Legacy behavior: append branch name as subdirectory
			branchPath = filepath.Join(path, branch)
		} else {
			// New default: use the path exactly as provided
			branchPath = path
		}
	} else {
		gitPath, err := getCommonGitDirectory()
		if err != nil {
			return fmt.Errorf("could not get working directory: %w", err)
		}

		branchPath = filepath.Join(gitPath, branch)
	}

	cmdArgs := []string{"worktree", "add", branchPath}

	_, err := git(cmdArgs)
	return err
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
