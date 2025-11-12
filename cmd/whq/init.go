package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var (
	initForce bool

	initCmd = &cobra.Command{
		Use:   "init",
		Short: "Generate a .whq.json template in the repo root",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return errors.New("Usage: whq init")
			}
			return runInit(initForce, os.Stdout, os.Stderr)
		},
	}
)

const defaultWHQTemplate = `{
  "post_add": {
    "copy": [],
    "commands": []
  }
}
`

func init() {
	initCmd.Flags().BoolVarP(&initForce, "force", "f", false, "Overwrite existing .whq.json")
}

func runInit(force bool, stdout, stderr io.Writer) error {
	repoRoot := env.RepoRoot
	if strings.TrimSpace(repoRoot) == "" {
		return errors.New("whq: not inside a Git repository")
	}

	configPath := filepath.Join(repoRoot, ".whq.json")
	if info, err := os.Stat(configPath); err == nil {
		if !force {
			fmt.Fprintln(stderr, ".whq.json already exists; use --force to overwrite")
			return errors.New("whq: .whq.json already exists (use --force to overwrite)")
		}
		if info.IsDir() {
			return errors.New("whq: .whq.json exists and is a directory")
		}
		fmt.Fprintln(stderr, "Overwriting existing .whq.json (--force)")
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("whq: failed to inspect .whq.json: %w", err)
	}

	if err := os.WriteFile(configPath, []byte(defaultWHQTemplate), 0o644); err != nil {
		return fmt.Errorf("whq: failed to write .whq.json: %w", err)
	}

	fmt.Fprintln(stdout, "Created .whq.json")
	return nil
}
