package tui

import "github.com/zhu13818202655/troupe/pkg/gitutil"

// detectGitBranch returns the active git branch for dir, delegating to
// pkg/gitutil. Returns empty string for non-git directories or when git
// is unavailable.
func detectGitBranch(dir string) string {
	return gitutil.DetectBranch(dir)
}
