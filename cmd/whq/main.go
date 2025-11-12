package main

import (
	"bufio"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

type whqEnv struct {
	RepoRoot    string
	WHQRoot     string
	Host        string
	Owner       string
	Project     string
	RepoWHQRoot string
}

var (
	version = "v0.0.4"

	rootCmd = &cobra.Command{
		Use:           "whq",
		Short:         "Minimal helper around git worktree",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Allow `whq help` to work outside a Git repo.
			if cmd.Name() == "help" || cmd.Name() == "version" {
				return nil
			}
			return initEnv()
		},
	}

	env whqEnv
)

func main() {
	// Subcommands
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(pathCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(lsCmd) // alias
	rootCmd.AddCommand(rmCmd)
	rootCmd.AddCommand(pruneCmd)
	rootCmd.AddCommand(rootPathCmd)
	rootCmd.AddCommand(versionCmd)

	if err := rootCmd.Execute(); err != nil {
		if msg := strings.TrimSpace(err.Error()); msg != "" {
			fmt.Fprintln(os.Stderr, msg)
		}
		os.Exit(1)
	}
}

// ----------------------
// Environment detection
// ----------------------

func initEnv() error {
	if _, err := exec.LookPath("git"); err != nil {
		return errors.New("whq: git is not available on PATH")
	}

	repoRoot, err := detectRepoRoot()
	if err != nil {
		return err
	}
	env.RepoRoot = repoRoot

	whqRoot := os.Getenv("WHQ_ROOT")
	if strings.TrimSpace(whqRoot) == "" {
		home, _ := os.UserHomeDir()
		if home == "" {
			return errors.New("whq: cannot determine home directory for WHQ_ROOT default")
		}
		whqRoot = filepath.Join(home, "whq")
	} else {
		// Expand leading ~ if present
		if strings.HasPrefix(whqRoot, "~") {
			home, _ := os.UserHomeDir()
			if home != "" {
				whqRoot = filepath.Join(home, strings.TrimPrefix(whqRoot, "~"))
			}
		}
	}
	absWHQ, _ := filepath.Abs(whqRoot)
	env.WHQRoot = absWHQ

	host, owner, project, err := detectRepoIdentity(repoRoot)
	if err != nil {
		return err
	}
	env.Host, env.Owner, env.Project = host, owner, project
	env.RepoWHQRoot = filepath.Join(env.WHQRoot, env.Host, env.Owner, env.Project)

	if err := os.MkdirAll(env.RepoWHQRoot, 0o755); err != nil {
		return fmt.Errorf("whq: failed to prepare repo worktrees root: %w", err)
	}

	return nil
}

func detectRepoRoot() (string, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = "."
	out, err := cmd.Output()
	if err != nil {
		// If not a git repo
		return "", errors.New("whq: not inside a Git repository")
	}
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "worktree ") {
			path := strings.TrimSpace(strings.TrimPrefix(line, "worktree "))
			if path == "" {
				break
			}
			abs, _ := filepath.Abs(path)
			return abs, nil
		}
	}
	return "", errors.New("whq: not inside a Git repository")
}

func detectRepoIdentity(repoRoot string) (string, string, string, error) {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return "", "", "", errors.New("whq: cannot determine repository identity (missing or invalid origin remote)")
	}
	urlStr := strings.TrimSpace(string(out))
	host, owner, project, perr := parseOriginURL(urlStr)
	if perr != nil || host == "" || owner == "" || project == "" {
		return "", "", "", errors.New("whq: cannot determine repository identity (missing or invalid origin remote)")
	}
	return host, owner, project, nil
}

func parseOriginURL(s string) (host, owner, project string, err error) {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, ".git")

	// git@host:owner/project
	if strings.HasPrefix(s, "git@") {
		// git@host:owner/project
		at := strings.IndexByte(s, '@')
		if at < 0 || at+1 >= len(s) {
			return "", "", "", fmt.Errorf("invalid SSH URL: %s", s)
		}
		rest := s[at+1:]
		// host:owner/project
		colon := strings.IndexByte(rest, ':')
		if colon < 0 || colon+1 >= len(rest) {
			return "", "", "", fmt.Errorf("invalid SSH URL: %s", s)
		}
		host = rest[:colon]
		path := rest[colon+1:]
		parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
		if len(parts) < 2 {
			return "", "", "", fmt.Errorf("invalid SSH URL path: %s", s)
		}
		owner = parts[0]
		project = parts[1]
		return
	}

	// https:// or http:// or ssh://
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "ssh://") {
		u, perr := url.Parse(s)
		if perr != nil {
			return "", "", "", perr
		}
		host = u.Hostname()
		path := strings.TrimPrefix(u.Path, "/")
		parts := strings.Split(path, "/")
		if len(parts) < 2 {
			return "", "", "", fmt.Errorf("invalid URL path: %s", s)
		}
		owner = parts[0]
		project = parts[1]
		return
	}

	// scp-like: host:owner/project
	if idx := strings.IndexByte(s, ':'); idx > 0 {
		host = s[:idx]
		path := s[idx+1:]
		parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
		if len(parts) < 2 {
			return "", "", "", fmt.Errorf("invalid scp-like URL: %s", s)
		}
		owner = parts[0]
		project = parts[1]
		return
	}

	return "", "", "", fmt.Errorf("unsupported origin URL: %s", s)
}

// ----------------------
// Subcommands
// ----------------------

var addCmd = &cobra.Command{
	Use:   "add <branch>",
	Short: "Create a new worktree for a branch",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return errors.New("Usage: whq add <branch>")
		}
		branch := args[0]
		dest := filepath.Join(env.RepoWHQRoot, branch)

		exists, err := branchExists(env.RepoRoot, branch)
		if err != nil {
			return err
		}

		var c *exec.Cmd
		if exists {
			c = exec.Command("git", "worktree", "add", dest, branch)
		} else {
			c = exec.Command("git", "worktree", "add", "-b", branch, dest)
		}
		c.Dir = env.RepoRoot
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		if err := c.Run(); err != nil {
			// git error is already printed to stderr
			return fmt.Errorf("")
		}

		if err := runPostAddActions(env.RepoRoot, dest); err != nil {
			if cleanupErr := cleanupFailedAdd(env.RepoRoot, dest, branch, !exists); cleanupErr != nil {
				return fmt.Errorf("%w; cleanup failed: %v", err, cleanupErr)
			}
			return err
		}

		fmt.Fprintf(os.Stdout, "Created worktree: %s\n", dest)
		return nil
	},
}

var pathCmd = &cobra.Command{
	Use:   "path <branch|@>",
	Short: "Print absolute path to worktree or root",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return errors.New("Usage: whq path <branch|@>")
		}
		arg := args[0]
		if arg == "@" {
			fmt.Fprintln(os.Stdout, env.RepoRoot)
			return nil
		}
		dest := filepath.Join(env.RepoWHQRoot, arg)
		if st, err := os.Stat(dest); err != nil || !st.IsDir() {
			return fmt.Errorf("whq: worktree '%s' not found at %s", arg, dest)
		}
		fmt.Fprintln(os.Stdout, dest)
		return nil
	},
}

var listPaths bool

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List worktrees for the repository",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get full list
		wts, err := listWorktrees(env.RepoRoot)
		if err != nil {
			return err
		}
		for i, p := range wts {
			if listPaths {
				fmt.Fprintln(os.Stdout, p)
				continue
			}
			if i == 0 {
				fmt.Fprintln(os.Stdout, "@")
				continue
			}
			// Relative to repo_whq_root if possible
			rel := tryRel(env.RepoWHQRoot, p)
			if rel == "" || strings.HasPrefix(rel, "..") {
				fmt.Fprintln(os.Stdout, p)
			} else {
				fmt.Fprintln(os.Stdout, rel)
			}
		}
		return nil
	},
}

var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "Alias for list",
	RunE:  listCmd.RunE,
}

func init() {
	listCmd.Flags().BoolVarP(&listPaths, "paths", "p", false, "Print absolute paths")
	lsCmd.Flags().BoolVarP(&listPaths, "paths", "p", false, "Print absolute paths")
}

var (
	rmForce  bool
	rmBranch bool
)

var rmCmd = &cobra.Command{
	Use:   "rm [-f|--force] [-b|--branch] <branch>",
	Short: "Remove a worktree (and optionally its branch)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return errors.New("Usage: whq rm [-f|--force] [-b|--branch] <branch>")
		}
		branch := args[0]
		dest := filepath.Join(env.RepoWHQRoot, branch)

		if err := removeWorktree(env.RepoRoot, dest, rmForce); err != nil {
			return fmt.Errorf("")
		}

		if rmBranch {
			if err := deleteBranch(env.RepoRoot, branch); err != nil {
				return fmt.Errorf("")
			}
		}
		return nil
	},
}

func init() {
	rmCmd.Flags().BoolVarP(&rmForce, "force", "f", false, "Force removal of worktree")
	rmCmd.Flags().BoolVarP(&rmBranch, "branch", "b", false, "Also delete the local branch")
}

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Run git worktree prune",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := exec.Command("git", "worktree", "prune")
		c.Dir = env.RepoRoot
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		if err := c.Run(); err != nil {
			return fmt.Errorf("")
		}
		return nil
	},
}

var rootPathCmd = &cobra.Command{
	Use:   "root",
	Short: "Print the repository's whq root",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(os.Stdout, env.RepoWHQRoot)
		return nil
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print whq version",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 0 {
			return errors.New("Usage: whq version")
		}
		fmt.Fprintln(os.Stdout, version)
		return nil
	},
}

// ----------------------
// Helpers
// ----------------------

func branchExists(repoRoot, branch string) (bool, error) {
	c := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	c.Dir = repoRoot
	if err := c.Run(); err != nil {
		// Non-zero exit likely means it doesn't exist
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() != 0 {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func listWorktrees(repoRoot string) ([]string, error) {
	c := exec.Command("git", "worktree", "list", "--porcelain")
	c.Dir = repoRoot
	out, err := c.Output()
	if err != nil {
		return nil, errors.New("whq: not inside a Git repository")
	}
	var paths []string
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "worktree ") {
			path := strings.TrimSpace(strings.TrimPrefix(line, "worktree "))
			abs, _ := filepath.Abs(path)
			paths = append(paths, abs)
		}
	}
	return paths, nil
}

func tryRel(base, target string) string {
	// Return relative path if target is within base, else empty string.
	b := filepath.Clean(base)
	t := filepath.Clean(target)
	rel, err := filepath.Rel(b, t)
	if err != nil {
		return ""
	}
	return rel
}

func removeWorktree(repoRoot, worktreePath string, force bool) error {
	argsWT := []string{"worktree", "remove"}
	if force {
		argsWT = append(argsWT, "--force")
	}
	argsWT = append(argsWT, worktreePath)
	c := exec.Command("git", argsWT...)
	c.Dir = repoRoot
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func deleteBranch(repoRoot, branch string) error {
	cb := exec.Command("git", "branch", "-d", branch)
	cb.Dir = repoRoot
	cb.Stdout = os.Stdout
	cb.Stderr = os.Stderr
	if err := cb.Run(); err != nil {
		cb2 := exec.Command("git", "branch", "-D", branch)
		cb2.Dir = repoRoot
		cb2.Stdout = os.Stdout
		cb2.Stderr = os.Stderr
		if err2 := cb2.Run(); err2 != nil {
			return err2
		}
	}
	return nil
}
