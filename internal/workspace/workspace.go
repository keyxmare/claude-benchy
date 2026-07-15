// Package workspace prepares an isolated copy of the application under test,
// overlays a configuration bundle onto it and records a git baseline so that
// changes made by Claude can later be captured as a diff.
package workspace

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// Prepare copies app into dst, overlays bundle on top of it and commits the
// result as the git baseline. The baseline includes the bundle so that a later
// diff only surfaces changes made to the application itself.
func Prepare(app, bundle, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	if err := copyTree(app, dst); err != nil {
		return fmt.Errorf("copy app: %w", err)
	}
	if err := copyTree(bundle, dst); err != nil {
		return fmt.Errorf("overlay bundle: %w", err)
	}
	if err := gitBaseline(dst); err != nil {
		return fmt.Errorf("git baseline: %w", err)
	}
	return nil
}

func gitBaseline(dir string) error {
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME=benchy",
		"GIT_AUTHOR_EMAIL=benchy@localhost",
		"GIT_COMMITTER_NAME=benchy",
		"GIT_COMMITTER_EMAIL=benchy@localhost",
	)
	steps := [][]string{
		{"init", "-q"},
		{"add", "-A"},
		{"commit", "-q", "--no-gpg-sign", "-m", "baseline"},
	}
	for _, args := range steps {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git %v: %w: %s", args, err, out)
		}
	}
	return nil
}

func copyTree(src, dst string) error {
	root := filepath.Clean(src)
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		// Never copy an existing VCS directory from the source tree.
		if d.IsDir() && (d.Name() == ".git") {
			return filepath.SkipDir
		}
		target := filepath.Join(dst, rel)

		switch {
		case d.IsDir():
			info, err := d.Info()
			if err != nil {
				return err
			}
			return os.MkdirAll(target, info.Mode().Perm())
		case d.Type()&os.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			_ = os.Remove(target)
			return os.Symlink(link, target)
		default:
			return copyFile(path, target)
		}
	})
}

func copyFile(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
