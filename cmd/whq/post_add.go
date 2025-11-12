package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type whqConfig struct {
	PostAdd *postAddConfig `json:"post_add"`
}

type postAddConfig struct {
	Copy     []string `json:"copy"`
	Commands []string `json:"commands"`
}

func runPostAddActions(repoRoot, worktreeRoot string) error {
	cfg, err := loadWHQConfig(repoRoot)
	if err != nil {
		return err
	}
	if cfg == nil || cfg.PostAdd == nil {
		return nil
	}

	copyTotal := len(cfg.PostAdd.Copy)
	cmdTotal := len(cfg.PostAdd.Commands)
	if copyTotal == 0 && cmdTotal == 0 {
		return nil
	}

	fmt.Fprintf(os.Stdout, "Post-add (.whq.json): starting (copy=%d, commands=%d)\n", copyTotal, cmdTotal)

	if err := executePostAddCopies(cfg.PostAdd.Copy, repoRoot, worktreeRoot); err != nil {
		return err
	}
	if err := executePostAddCommands(cfg.PostAdd.Commands, worktreeRoot); err != nil {
		return err
	}

	fmt.Fprintln(os.Stdout, "Post-add (.whq.json): completed")
	return nil
}

func loadWHQConfig(repoRoot string) (*whqConfig, error) {
	cfgPath := filepath.Join(repoRoot, ".whq.json")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("whq: failed to read .whq.json: %w", err)
	}
	var cfg whqConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("whq: invalid .whq.json: %w", err)
	}
	return &cfg, nil
}

func executePostAddCopies(entries []string, repoRoot, worktreeRoot string) error {
	if len(entries) == 0 {
		return nil
	}
	for _, raw := range entries {
		item := strings.TrimSpace(raw)
		if item == "" {
			return errors.New("whq: post-add copy entry cannot be empty")
		}

		src, err := safeJoin(repoRoot, item)
		if err != nil {
			return fmt.Errorf("whq: invalid post-add copy path %q: %w", item, err)
		}
		if _, err := os.Lstat(src); err != nil {
			return fmt.Errorf("whq: post-add copy source %q not found: %w", item, err)
		}

		dest, err := safeJoin(worktreeRoot, item)
		if err != nil {
			return fmt.Errorf("whq: invalid destination for copy %q: %w", item, err)
		}

		fmt.Fprintf(os.Stdout, "Post-add copy: %s\n", item)
		if err := copyPath(src, dest); err != nil {
			return fmt.Errorf("whq: failed to copy %q: %w", item, err)
		}
	}
	return nil
}

func executePostAddCommands(commands []string, worktreeRoot string) error {
	if len(commands) == 0 {
		return nil
	}
	for i, raw := range commands {
		cmdStr := strings.TrimSpace(raw)
		if cmdStr == "" {
			return fmt.Errorf("whq: post-add command %d is empty", i+1)
		}

		fmt.Fprintf(os.Stdout, "Post-add cmd %d/%d: %s\n", i+1, len(commands), cmdStr)
		cmd := exec.Command("bash", "-lc", cmdStr)
		cmd.Dir = worktreeRoot
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("whq: post-add command failed (%s): %w", cmdStr, err)
		}
	}
	return nil
}

func safeJoin(root, rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("path must be relative")
	}
	clean := filepath.Clean(rel)
	if clean == "." || clean == "" {
		return "", fmt.Errorf("path resolves to root")
	}
	if strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
		return "", fmt.Errorf("path escapes repository root")
	}
	return filepath.Join(root, clean), nil
}

func copyPath(src, dest string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return copySymlink(src, dest)
	}
	if info.IsDir() {
		return copyDirectory(src, dest)
	}
	return copyFile(src, dest, info.Mode().Perm())
}

func copySymlink(src, dest string) error {
	if err := os.RemoveAll(dest); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	target, err := os.Readlink(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	return os.Symlink(target, dest)
}

func copyDirectory(src, dest string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("source is not a directory: %s", src)
	}
	if err := os.RemoveAll(dest); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(dest, info.Mode().Perm()); err != nil {
		return err
	}
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == src {
			return nil
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			return os.MkdirAll(target, info.Mode().Perm())
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return copySymlink(path, target)
		}
		return copyFile(path, target, info.Mode().Perm())
	})
}

func copyFile(src, dest string, perm fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	if err := os.RemoveAll(dest); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer func() {
		_ = out.Close()
	}()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}

func cleanupFailedAdd(repoRoot, worktreeRoot, branch string, removeBranch bool) error {
	fmt.Fprintf(os.Stderr, "Post-add failed: removing worktree %s\n", worktreeRoot)

	if err := removeWorktree(repoRoot, worktreeRoot, false); err != nil {
		return fmt.Errorf("whq: cleanup failed while removing worktree: %w", err)
	}
	_ = os.RemoveAll(worktreeRoot)

	if removeBranch {
		fmt.Fprintf(os.Stderr, "Post-add failed: deleting branch %s\n", branch)
		if err := deleteBranch(repoRoot, branch); err != nil {
			return fmt.Errorf("whq: cleanup failed while deleting branch: %w", err)
		}
	}
	return nil
}
