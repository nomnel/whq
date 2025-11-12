package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunPostAddActionsCopiesAndCommands(t *testing.T) {
	repoRoot := t.TempDir()
	worktreeRoot := t.TempDir()

	writeFile(t, filepath.Join(repoRoot, ".whq.json"), `{
		"post_add": {
			"copy": ["configs/app.env", "scripts"],
			"commands": ["printf 'ready' > ready.txt"]
		}
	}`)

	writeFile(t, filepath.Join(repoRoot, "configs", "app.env"), "FOO=bar\n")
	writeFile(t, filepath.Join(repoRoot, "scripts", "setup.sh"), "#!/bin/sh\necho ok\n")

	if err := runPostAddActions(repoRoot, worktreeRoot); err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	gotConfig, err := os.ReadFile(filepath.Join(worktreeRoot, "configs", "app.env"))
	if err != nil {
		t.Fatalf("expected copied config file: %v", err)
	}
	if string(gotConfig) != "FOO=bar\n" {
		t.Fatalf("config mismatch: %q", gotConfig)
	}

	gotScript, err := os.ReadFile(filepath.Join(worktreeRoot, "scripts", "setup.sh"))
	if err != nil {
		t.Fatalf("expected copied script: %v", err)
	}
	if !strings.Contains(string(gotScript), "echo ok") {
		t.Fatalf("script mismatch: %q", gotScript)
	}

	if _, err := os.Stat(filepath.Join(worktreeRoot, "ready.txt")); err != nil {
		t.Fatalf("expected command output file: %v", err)
	}
}

func TestRunPostAddActionsMissingCopy(t *testing.T) {
	repoRoot := t.TempDir()
	worktreeRoot := t.TempDir()

	writeFile(t, filepath.Join(repoRoot, ".whq.json"), `{
		"post_add": {
			"copy": ["missing.txt"]
		}
	}`)

	err := runPostAddActions(repoRoot, worktreeRoot)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got %v", err)
	}
}

func TestRunPostAddActionsCommandFailure(t *testing.T) {
	repoRoot := t.TempDir()
	worktreeRoot := t.TempDir()

	writeFile(t, filepath.Join(repoRoot, ".whq.json"), `{
		"post_add": {
			"commands": ["exit 9"]
		}
	}`)

	err := runPostAddActions(repoRoot, worktreeRoot)
	if err == nil || !strings.Contains(err.Error(), "post-add command failed") {
		t.Fatalf("expected command failure, got %v", err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
}
