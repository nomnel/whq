package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCleanupFailedAddRemovesWorktreeAndBranch(t *testing.T) {
	repoRoot := t.TempDir()
	worktreeRoot := filepath.Join(repoRoot, "wt-feature")
	if err := os.MkdirAll(worktreeRoot, 0o755); err != nil {
		t.Fatalf("setup mkdir failed: %v", err)
	}

	logFile := filepath.Join(repoRoot, "git.log")
	fakeGitDir := t.TempDir()
	scriptPath := filepath.Join(fakeGitDir, "git")
	script := fmt.Sprintf(`#!/bin/sh
echo "$@" >> %s
if [ "$1" = "worktree" ] && [ "$2" = "remove" ]; then
  exit 0
fi
if [ "$1" = "branch" ] && [ "$2" = "-d" ]; then
  exit 1
fi
exit 0
`, logFile)
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake git failed: %v", err)
	}

	origPath := os.Getenv("PATH")
	os.Setenv("PATH", fakeGitDir+string(os.PathListSeparator)+origPath)
	t.Cleanup(func() { os.Setenv("PATH", origPath) })

	if err := cleanupFailedAdd(repoRoot, worktreeRoot, "feature/test", true); err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}

	if _, err := os.Stat(worktreeRoot); !os.IsNotExist(err) {
		t.Fatalf("expected worktree directory removed, err=%v", err)
	}

	logData, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("read log failed: %v", err)
	}
	log := string(logData)
	if !strings.Contains(log, "worktree remove") {
		t.Fatalf("expected worktree remove command, log=%q", log)
	}
	if !strings.Contains(log, "branch -d") || !strings.Contains(log, "branch -D") {
		t.Fatalf("expected branch deletion attempts, log=%q", log)
	}
}

func TestCleanupFailedAddSkipsBranchDeletionWhenNotRequested(t *testing.T) {
	repoRoot := t.TempDir()
	worktreeRoot := filepath.Join(repoRoot, "wt-existing")
	if err := os.MkdirAll(worktreeRoot, 0o755); err != nil {
		t.Fatalf("setup mkdir failed: %v", err)
	}

	logFile := filepath.Join(repoRoot, "git.log")
	fakeGitDir := t.TempDir()
	scriptPath := filepath.Join(fakeGitDir, "git")
	script := fmt.Sprintf(`#!/bin/sh
echo "$@" >> %s
exit 0
`, logFile)
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake git failed: %v", err)
	}

	origPath := os.Getenv("PATH")
	os.Setenv("PATH", fakeGitDir+string(os.PathListSeparator)+origPath)
	t.Cleanup(func() { os.Setenv("PATH", origPath) })

	if err := cleanupFailedAdd(repoRoot, worktreeRoot, "feature/existing", false); err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}

	logData, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("read log failed: %v", err)
	}
	log := string(logData)
	if strings.Contains(log, "branch") {
		t.Fatalf("did not expect branch deletion command, log=%q", log)
	}
}
