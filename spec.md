# whq Command Specification

This document defines the behavior of the `whq` command to be implemented in Go.
Unless otherwise stated, `whq` interacts with the current Git repository only
(no cross-repo scope).

## Overview

- Purpose: Minimal helper around `git worktree` for a single repository.
- Scope: Operates only when invoked inside a Git repository; fails otherwise.
- Worktree layout: Additional worktrees are created outside `.git`, following a
  ghq-like layout: `WHQ_ROOT/<host>/<owner>/<project>/<worktree>`. By default,
  `WHQ_ROOT` is `~/whq` and can be overridden by the `WHQ_ROOT` environment
  variable.
- Repository root: Determined as the first entry of
  `git worktree list --porcelain` (the “main” worktree). This absolute path is
  referred to as `repo_root`.
- Repository worktrees root:
  `repo_whq_root = WHQ_ROOT/<host>/<owner>/<project>`, where
  `<host>/<owner>/<project>` is derived from the `origin` remote URL.
- Root alias: `@` denotes the repository’s main worktree directory (`repo_root`)
  in listings and for `path`.

## Common Behavior

- Preconditions: `git` must be available on `PATH`. The current directory must
  be within a Git worktree of the target repository.
- Initialization:
  - Determine `repo_root` using `git worktree list --porcelain` and reading the
    first `worktree` entry.
  - Determine `WHQ_ROOT` from the `WHQ_ROOT` env var, defaulting to `~/whq`.
  - Resolve repository identity from `git remote get-url origin` to obtain
    `<host>/<owner>/<project>`.
  - Compute `repo_whq_root = WHQ_ROOT/<host>/<owner>/<project>` and create it if
    it does not exist.
- Errors:
  - If not inside a Git repository, exit with a non-zero code and message:
    `whq: not inside a Git repository`.
  - For invalid usage, print a brief usage message for the subcommand and exit
    non-zero.
- Side effects:
  - `whq add` runs `git worktree add` and thereby triggers standard Git hooks in
    the created worktree.

## whq add

- Synopsis: `whq add <branch>`
- Description:
  - Creates a new worktree at `repo_whq_root/<branch>`.
  - If a local branch named `<branch>` already exists (`refs/heads/<branch>`),
    run: `git worktree add <dest> <branch>`.
  - If it does not exist, create it from the current HEAD:
    `git worktree add -b <branch> <dest>`.
- Arguments:
  - `<branch>`: Branch name to use for the worktree directory and (if not
    existing) the new branch.
- Options: (none)
- Output:
  - On success: `Created worktree: <dest>`.
- Errors:
  - Missing `<branch>`: print `Usage: whq add <branch>` and exit non-zero.
  - Any failure from `git worktree add` should cause a non-zero exit; surface
    Git’s error output.

## whq path

- Synopsis: `whq path <branch|@>`
- Description:
  - Prints the absolute path to the specified worktree or to the repository
    root.
  - Special case: `@` prints `repo_root` (the main worktree directory).
- Arguments:
  - `<branch>`: Branch name corresponding to `repo_whq_root/<branch>`.
  - `@`: Alias for the repository root (`repo_root`).
- Output:
  - Absolute path only (no prefix text), with a trailing newline.
- Errors:
  - Missing argument: `Usage: whq path <branch|@>` and exit non-zero.
  - If `repo_whq_root/<branch>` does not exist:
    `whq: worktree '<branch>' not found at <dest>` and exit non-zero.

## whq list / whq ls

- Synopsis: `whq list [-p]` (alias: `whq ls [-p]`)
- Description:
  - Lists all worktrees for the current repository using
    `git worktree list --porcelain`.
  - Each `worktree` path is printed on its own line.
  - Default output prints `@` for the main worktree, and for other worktrees
    prints paths relative to `repo_whq_root` when possible; if a worktree path
    is not under `repo_whq_root`, print the absolute path.
- Options:
  - `-p`: Print full absolute paths for all worktrees (no `@` special-casing).
- Output examples (relative mode):
- `@` (for the main worktree)
- `feature-123`
- `bugfix-x`

## whq rm

- Synopsis: `whq rm [-f|--force] [-b|--branch] <branch>`
- Description:
  - Removes the worktree at `repo_whq_root/<branch>` using
    `git worktree remove`.
  - If `-b`/`--branch` is specified, also delete the local branch `<branch>`:
    - Attempt `git branch -d <branch>`, falling back to `git branch -D <branch>`
      if the branch is not fully merged.
- Arguments:
  - `<branch>`: Name of the worktree directory under `repo_whq_root` to remove,
    and the branch to delete if `-b` is provided.
- Options:
  - `-f`, `--force`: Pass `--force` to `git worktree remove`.
  - `-b`, `--branch`: Also delete the local branch after removing the worktree.
- Errors:
  - Missing `<branch>`: `Usage: whq rm [-f|--force] [-b|--branch] <branch>` and
    exit non-zero.
  - Any failures from `git worktree remove` or `git branch` propagate as
    non-zero exits with Git’s error messages.

## whq prune

- Synopsis: `whq prune`
- Description: Runs `git worktree prune` to clean up stale/invalid worktrees.
- Output: Surface Git’s output.

## whq root

- Synopsis: `whq root`
- Description: Prints the absolute path to the repository’s worktrees root under
  `WHQ_ROOT`: `repo_whq_root`.

## whq version

- Synopsis: `whq version`
- Description: Prints the CLI version string to stdout. The default build value
  is `v0.0.2` and can be overridden at build time via
  `-ldflags "-X main.version=<value>"`.
- Output: Version string only (no prefix text), with a trailing newline.
- Errors:
  - Any arguments present: print `Usage: whq version` and exit non-zero.

## whq help

- Synopsis: `whq help`
- Description: Prints a usage summary of all commands and options.

## Exit Codes

- `0` on success.
- Non-zero on errors (invalid usage, not in a Git repo, Git command failures, or
  failed directory changes).

## Implementation Notes (Go)

- CLI framework:
  - Use `spf13/cobra` to implement the `whq` command with subcommands (`add`,
    `path`, `list`, `ls` alias, `rm`, `prune`, `root`, `version`, and `help`).
    Cobra handles argument/flag parsing, usage, and `-h/--help` output.
  - Optionally, leverage Cobra’s completion generation to provide shell
    completion scripts.
- Detection of `repo_root`:
  - Execute `git worktree list --porcelain`; parse the first `worktree <path>`
    line and take `<path>` as `repo_root`.
- Detection of `repo_root`:
  - Execute `git worktree list --porcelain`; parse the first `worktree <path>`
    line and take `<path>` as `repo_root`.
- `WHQ_ROOT` and repository identity:
  - Read `WHQ_ROOT` from the environment, defaulting to `~/whq`.
  - Resolve repository identity via `git remote get-url origin`, parsing into
    `<host>/<owner>/<project>`.
  - Supported forms include SSH (`git@host:owner/project(.git)`) and HTTPS
    (`https://host/owner/project(.git)`). Strip a trailing `.git`.
  - If `origin` is missing or cannot be parsed, exit with a clear error:
    `whq: cannot determine repository identity (missing or invalid origin remote)`.
- Worktree operations:
  - Use `git` subcommands (`worktree add/remove/prune`, `show-ref`, `branch`)
    via subprocess execution and propagate return codes and stderr.
  - Ensure `repo_whq_root = WHQ_ROOT/<host>/<owner>/<project>` exists before
    creating worktrees.
- Path handling:
  - Use absolute paths when interacting with Git.
  - For `path`, print the absolute path only.
  - For `list` relative mode, print `@` for `repo_root`. For other entries, if
    the path starts with `repo_whq_root + "/"`, trim that prefix and print the
    remainder; otherwise print the absolute path.
- Messages:
  - Match the strings described above to preserve UX parity.
- No built-in `cd`:
  - The Go CLI does not change the parent shell’s CWD. Use `whq path <branch|@>`
    to obtain the absolute path and combine with shell features if desired,
    e.g., `cd "$(whq path feature-123)"`.

## Shell Completion (Optional)

- Implementations may provide shell completion scripts for common shells.
- Suggested behavior:
  - `path` and `rm`: complete from subdirectories of `repo_whq_root` plus `@`
    for the repository root.
  - `add`: complete from existing local branches.
