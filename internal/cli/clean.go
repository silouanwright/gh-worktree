package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	gh "github.com/cli/go-gh"
	"github.com/cli/safeexec"
	"github.com/spf13/cobra"
)

type WorktreeInfo struct {
	Path       string
	Branch     string
	PRNumber   int
	LastCommit time.Time
	PRStatus   string // "open", "merged", "closed", or ""
}

func NewClean() *cobra.Command {
	var dryRun bool
	var staleDays int

	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Clean up worktrees for merged/closed PRs and identify stale worktrees",
		Long: `Automatically removes worktrees for merged or closed PRs.
Lists stale worktrees (no commits in 30+ days) for manual review.`,
		Example: "gh worktree clean",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("🔍 Analyzing worktrees...")

			worktrees, err := getWorktreeInfo()
			if err != nil {
				return fmt.Errorf("failed to get worktree info: %w", err)
			}

			if len(worktrees) == 0 {
				fmt.Println("No worktrees found besides main.")
				return nil
			}

			repo, err := gh.CurrentRepository()
			if err != nil {
				fmt.Println("⚠️  Could not get current repository - skipping PR status checks")
			}

			var toRemove []WorktreeInfo
			var staleWorktrees []WorktreeInfo

			for _, wt := range worktrees {
				// Skip main worktree
				if strings.Contains(wt.Path, "/.git") || wt.Branch == "main" || wt.Branch == "master" {
					continue
				}

				// Check PR status if we have a PR number
				if wt.PRNumber > 0 && repo != nil {
					status, err := getPRStatus(repo, wt.PRNumber)
					if err == nil {
						wt.PRStatus = status
						if status == "merged" || status == "closed" {
							toRemove = append(toRemove, wt)
							continue
						}
					}
				}

				// Check for stale worktrees
				daysSinceCommit := int(time.Since(wt.LastCommit).Hours() / 24)
				if daysSinceCommit > staleDays {
					staleWorktrees = append(staleWorktrees, wt)
				}
			}

			// Remove merged/closed PR worktrees
			if len(toRemove) > 0 {
				fmt.Printf("\n🧹 Found %d worktree(s) for merged/closed PRs:\n\n", len(toRemove))
				for _, wt := range toRemove {
					fmt.Printf("  • %s (PR #%d - %s)\n", filepath.Base(wt.Path), wt.PRNumber, wt.PRStatus)
					if !dryRun {
						if err := removeWorktree(wt.Path); err != nil {
							fmt.Printf("    ❌ Failed to remove: %v\n", err)
						} else {
							fmt.Printf("    ✅ Removed\n")
						}
					}
				}
				if dryRun {
					fmt.Println("\n(Dry run - no worktrees were removed)")
				}
			}

			// Show stale worktrees for review
			if len(staleWorktrees) > 0 {
				fmt.Printf("\n📅 Found %d stale worktree(s) (no commits in %d+ days):\n\n", len(staleWorktrees), staleDays)
				for i, wt := range staleWorktrees {
					daysSince := int(time.Since(wt.LastCommit).Hours() / 24)
					fmt.Printf("  %d. %s (%s)\n", i+1, filepath.Base(wt.Path), wt.Branch)
					fmt.Printf("     Last commit: %d days ago\n", daysSince)
					if wt.PRNumber > 0 && wt.PRStatus != "" {
						fmt.Printf("     PR #%d (%s)\n", wt.PRNumber, wt.PRStatus)
					}
				}

				if !dryRun {
					fmt.Print("\nWould you like to remove any of these? Enter numbers separated by spaces (or 'all' for all, Enter to skip): ")
					reader := bufio.NewReader(os.Stdin)
					response, _ := reader.ReadString('\n')
					response = strings.TrimSpace(response)

					if response != "" {
						var toDelete []WorktreeInfo
						if response == "all" {
							toDelete = staleWorktrees
						} else {
							indices := strings.Fields(response)
							for _, idxStr := range indices {
								if idx, err := strconv.Atoi(idxStr); err == nil && idx > 0 && idx <= len(staleWorktrees) {
									toDelete = append(toDelete, staleWorktrees[idx-1])
								}
							}
						}

						for _, wt := range toDelete {
							if err := removeWorktree(wt.Path); err != nil {
								fmt.Printf("❌ Failed to remove %s: %v\n", filepath.Base(wt.Path), err)
							} else {
								fmt.Printf("✅ Removed %s\n", filepath.Base(wt.Path))
							}
						}
					}
				}
			}

			if len(toRemove) == 0 && len(staleWorktrees) == 0 {
				fmt.Println("✨ All worktrees are active and up to date!")
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be cleaned without actually removing")
	cmd.Flags().IntVar(&staleDays, "stale-days", 30, "Number of days without commits to consider a worktree stale")

	return cmd
}

func getWorktreeInfo() ([]WorktreeInfo, error) {
	git, err := safeexec.LookPath("git")
	if err != nil {
		return nil, err
	}

	// Get worktree list
	cmd := exec.Command(git, "worktree", "list", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var worktrees []WorktreeInfo
	lines := strings.Split(string(output), "\n")
	var current WorktreeInfo

	for _, line := range lines {
		if strings.HasPrefix(line, "worktree ") {
			if current.Path != "" {
				// Before appending, ensure PR number is extracted
				if current.PRNumber == 0 {
					current.PRNumber = extractPRNumber(filepath.Base(current.Path))
				}
				worktrees = append(worktrees, current)
			}
			current = WorktreeInfo{
				Path: strings.TrimPrefix(line, "worktree "),
			}
		} else if strings.HasPrefix(line, "branch refs/heads/") {
			current.Branch = strings.TrimPrefix(line, "branch refs/heads/")
			// Try to extract PR number from branch name
			current.PRNumber = extractPRNumber(current.Branch)
			// If no PR number found in branch, try the path
			if current.PRNumber == 0 {
				current.PRNumber = extractPRNumber(filepath.Base(current.Path))
			}
		} else if line == "" && current.Path != "" {
			// Before appending, ensure PR number is extracted
			if current.PRNumber == 0 {
				current.PRNumber = extractPRNumber(filepath.Base(current.Path))
			}
			worktrees = append(worktrees, current)
			current = WorktreeInfo{}
		}
	}
	// Handle last worktree if it exists
	if current.Path != "" {
		if current.PRNumber == 0 {
			current.PRNumber = extractPRNumber(filepath.Base(current.Path))
		}
		worktrees = append(worktrees, current)
	}

	// Get last commit date for each worktree
	for i := range worktrees {
		if worktrees[i].Branch != "" {
			lastCommit, err := getLastCommitDate(worktrees[i].Path)
			if err == nil {
				worktrees[i].LastCommit = lastCommit
			}
		}
	}

	return worktrees, nil
}

func extractPRNumber(text string) int {
	// Common patterns: pr-123, pr/123, pull/123, 123-feature, web-frontend-pr-1018
	// Check most specific patterns first
	patterns := []string{
		`[-_]pr[-_/](\d+)`,          // Matches -pr-123, _pr_123, -pr/123
		`^pr[-_/](\d+)`,             // Matches pr-123, pr_123, pr/123 at start
		`[-_]pull[-_/](\d+)`,        // Matches -pull-123, _pull_123
		`^pull[-_/](\d+)`,           // Matches pull-123, pull_123 at start
		`^(\d+)[-_]`,                // Matches 123-feature at start
		`[-_](\d{4,})$`,             // Matches feature-1234 at end (4+ digits to avoid false positives)
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(text); len(matches) > 1 {
			if num, err := strconv.Atoi(matches[1]); err == nil {
				return num
			}
		}
	}
	return 0
}

func getLastCommitDate(worktreePath string) (time.Time, error) {
	git, err := safeexec.LookPath("git")
	if err != nil {
		return time.Time{}, err
	}

	cmd := exec.Command(git, "-C", worktreePath, "log", "-1", "--format=%at")
	output, err := cmd.Output()
	if err != nil {
		return time.Time{}, err
	}

	timestamp := strings.TrimSpace(string(output))
	if timestamp == "" {
		return time.Time{}, fmt.Errorf("no commits found")
	}

	unix, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return time.Time{}, err
	}

	return time.Unix(unix, 0), nil
}

func getPRStatus(repo interface{ Owner() string; Name() string }, prNumber int) (string, error) {
	client, err := gh.RESTClient(nil)
	if err != nil {
		return "", err
	}

	var pr struct {
		State  string
		Merged bool `json:"merged"`
	}

	err = client.Get(fmt.Sprintf("repos/%s/%s/pulls/%d", repo.Owner(), repo.Name(), prNumber), &pr)
	if err != nil {
		return "", err
	}

	if pr.Merged {
		return "merged", nil
	}
	return pr.State, nil // "open" or "closed"
}

func removeWorktree(path string) error {
	git, err := safeexec.LookPath("git")
	if err != nil {
		return err
	}

	cmd := exec.Command(git, "worktree", "remove", path, "--force")
	return cmd.Run()
}