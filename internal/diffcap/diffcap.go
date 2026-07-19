// Package diffcap captures the working-tree changes of a prepared workspace as
// a unified diff against its git baseline.
package diffcap

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Stats summarises a diff.
type Stats struct {
	FilesChanged int
	Insertions   int
	Deletions    int
}

// Result is a captured diff and its stats.
type Result struct {
	Patch string
	Stats Stats
}

// Capture stages every change in dir and returns the diff against the baseline
// commit. It assumes dir is a git repository with a baseline commit.
func Capture(dir string) (Result, error) {
	if _, err := git(dir, "add", "-A"); err != nil {
		return Result{}, err
	}
	patch, err := git(dir, "diff", "--cached")
	if err != nil {
		return Result{}, err
	}
	numstat, err := git(dir, "diff", "--cached", "--numstat")
	if err != nil {
		return Result{}, err
	}
	return Result{Patch: patch, Stats: parseNumstat(numstat)}, nil
}

func parseNumstat(out string) Stats {
	var s Stats
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, "\t", 3)
		if len(fields) < 3 {
			continue
		}
		s.FilesChanged++
		if n, err := strconv.Atoi(fields[0]); err == nil {
			s.Insertions += n
		}
		if n, err := strconv.Atoi(fields[1]); err == nil {
			s.Deletions += n
		}
	}
	return s
}

func git(dir string, args ...string) (string, error) {
	// safe.directory=* disables git's ownership check: the dashboard runs git
	// (as root, in a container) over a bind-mounted workspace whose files git
	// sees as owned by another user, which otherwise aborts with "dubious
	// ownership". These workspaces are throwaway and fully owned by benchy.
	full := append([]string{"-c", "safe.directory=*", "-C", dir}, args...)
	cmd := exec.Command("git", full...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %v: %w: %s", args, err, out)
	}
	return string(out), nil
}
