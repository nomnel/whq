package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunInitCreatesTemplate(t *testing.T) {
	repo := t.TempDir()
	origEnv := env
	env.RepoRoot = repo
	t.Cleanup(func() { env = origEnv })

	var stdout, stderr bytes.Buffer
	if err := runInit(false, &stdout, &stderr); err != nil {
		t.Fatalf("runInit failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(repo, ".whq.json"))
	if err != nil {
		t.Fatalf(".whq.json not written: %v", err)
	}
	if string(data) != defaultWHQTemplate {
		t.Fatalf("template mismatch: %q", string(data))
	}

	if !strings.Contains(stdout.String(), "Created .whq.json") {
		t.Fatalf("stdout missing creation message: %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestRunInitSkipsWhenFileExistsWithoutForce(t *testing.T) {
	repo := t.TempDir()
	path := filepath.Join(repo, ".whq.json")
	if err := os.WriteFile(path, []byte("custom"), 0o644); err != nil {
		t.Fatalf("write seed file failed: %v", err)
	}
	origEnv := env
	env.RepoRoot = repo
	t.Cleanup(func() { env = origEnv })

	var stdout, stderr bytes.Buffer
	err := runInit(false, &stdout, &stderr)
	if err == nil {
		t.Fatalf("expected error when file exists")
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "already exists") {
		t.Fatalf("stderr missing skip message: %q", stderr.String())
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read existing file: %v", err)
	}
	if string(data) != "custom" {
		t.Fatalf("existing file should remain untouched: %q", string(data))
	}
}

func TestRunInitForceOverwritesExisting(t *testing.T) {
	repo := t.TempDir()
	path := filepath.Join(repo, ".whq.json")
	if err := os.WriteFile(path, []byte("outdated"), 0o600); err != nil {
		t.Fatalf("write seed file failed: %v", err)
	}
	origEnv := env
	env.RepoRoot = repo
	t.Cleanup(func() { env = origEnv })

	var stdout, stderr bytes.Buffer
	if err := runInit(true, &stdout, &stderr); err != nil {
		t.Fatalf("runInit with force failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf(".whq.json not written: %v", err)
	}
	if string(data) != defaultWHQTemplate {
		t.Fatalf("template mismatch: %q", string(data))
	}
	if !strings.Contains(stdout.String(), "Created .whq.json") {
		t.Fatalf("stdout missing creation message: %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "Overwriting") {
		t.Fatalf("stderr missing overwrite notice: %q", stderr.String())
	}
}
