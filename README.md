# whq

Minimal helper around `git worktree` for a single Git repository. It creates and
manages per-branch worktrees in a predictable, ghq-like layout outside of
`.git`.

- Operates only inside a Git repository; otherwise exits with:
  `whq: not inside a Git repository`.
- Worktrees live under `WHQ_ROOT/<host>/<owner>/<project>/<worktree>` (default
  `WHQ_ROOT=~/whq`).
- The main worktree is detected from `git worktree list --porcelain` and
  referred to as `@`.

## Install

- Using Go (requires Go 1.25+):
  - Latest: `go install github.com/nomnel/whq/cmd/whq@latest`
  - Specific: `go install github.com/nomnel/whq/cmd/whq@v0.0.1`
- From source (in this repo): `go build -o whq ./cmd/whq`

Ensure `git` is available on your `PATH`.

## Quick Start

- Create a worktree for a branch (creates the branch if it does not exist):
  - `whq add feature-123`
- Jump to a worktree directory:
  - `cd "$(whq path feature-123)"`
- Jump to the main worktree:
  - `cd "$(whq path @)"`
- List worktrees for the current repo:
  - `whq ls` (relative/`@` by default), or `whq ls -p` (absolute paths)
- Remove a worktree (and optionally its branch):
  - `whq rm feature-123`
  - `whq rm -b feature-123` (also delete the local branch)
- Prune stale entries:
  - `whq prune`
- Show the repo’s WHQ worktrees root:
  - `whq root`
- Print the CLI version:
  - `whq version`

## Behavior

- `repo_root`: First `worktree` entry from `git worktree list --porcelain`.
- `WHQ_ROOT`: From env var; defaults to `~/whq` (leading `~` expanded). The
  repository’s worktree root is `WHQ_ROOT/<host>/<owner>/<project>` derived from
  the `origin` remote (SSH, HTTPS, or scp-like). A trailing `.git` is stripped.
- All Git operations are executed via subprocesses; Git’s stderr is surfaced on
  failures.

## Commands

- `whq add <branch>`: Create `repo_whq_root/<branch>` worktree; creates branch
  from HEAD if missing.
- `whq path <branch|@>`: Print absolute path to a worktree, or `repo_root` for
  `@`.
- `whq list [-p]` / `whq ls [-p]`:
  - Default: `@` for main worktree, others relative to `repo_whq_root` when
    possible.
  - `-p`: Print absolute paths for all worktrees.
- `whq rm [-f|--force] [-b|--branch] <branch>`: Remove a worktree; with `-b`,
  also delete the local branch (`-d`, fallback to `-D`).
- `whq prune`: Run `git worktree prune`.
- `whq root`: Print `repo_whq_root`.
- `whq version`: Print the CLI version string (current default `v0.0.2`,
  overridable via `-ldflags`).

## Notes

- The CLI does not change your shell’s directory. Use `whq path` with shell
  substitution to `cd`.
- If `origin` is missing or cannot be parsed, `whq` exits with:
  - `whq: cannot determine repository identity (missing or invalid origin remote)`

## Examples

- Create and enter a feature worktree:
  - `whq add feature-123 && cd "$(whq path feature-123)"`
- List all worktrees (absolute):
  - `whq ls -p`
- Remove a worktree and its branch:
  - `whq rm -b feature-123`

## Shell helpers (optional)

- Bash/Zsh function to jump to a worktree (supports `@`):

```sh
whqcd() { cd "$(whq path "${1:-@}")"; }
```

---

See `spec.md` for the full specification and implementation details.

## Git hook: post-checkout (optional)

This repository’s `whq` CLI is designed to manage external worktrees. If you
want new worktrees to auto-initialize (e.g., copy some dotfiles and run setup
commands), you can install the following `post-checkout` hook in another
repository that uses `whq`-managed worktrees.

Notes:

- The hook runs only when `PWD` is under the repository’s `whq` root:
  `$(whq root)/*` (i.e., `WHQ_ROOT/<host>/<owner>/<project>/*`).
- It copies selected files from the main worktree (printed by `whq path @`) into
  the newly checked-out worktree, then runs optional setup commands.
- If `whq` is not available or the repository cannot be identified, the hook
  exits silently and does nothing.

### Setup

1. Install `whq` and ensure it’s on your `PATH`.
2. In the target repository (not this repo), save the script below as
   `.git/hooks/post-checkout`.
3. Make it executable: `chmod +x .git/hooks/post-checkout`.
4. Optionally customize `FILES_TO_COPY` and `COMMANDS` in the script.

### Script (.git/hooks/post-checkout)

```zsh
#!/usr/bin/env zsh
# post-checkout hook for whq-managed external worktrees
# - Runs only when $PWD is under "$(whq root)/*"
# - Copies selected files from the main worktree into the new worktree
# - Optionally runs per-worktree setup commands

set -u  # treat unset variables as an error; disable if undesired

# Ensure whq is available and the repo is recognized
if ! command -v whq >/dev/null 2>&1; then
  # whq is not available; do nothing
  exit 0
fi

# Repository's whq root (e.g., $WHQ_ROOT/<host>/<owner>/<project>)
repo_whq_root="$(whq root 2>/dev/null || true)"
if [[ -z "${repo_whq_root:-}" ]]; then
  # Not inside a recognizable repository; do nothing
  exit 0
fi

# Run only for whq-managed additional worktrees (usually excludes the main one)
case "$PWD" in
  "${repo_whq_root}"/*) ;;
  *) exit 0 ;;
esac

# Main worktree root ("@")
repo_root="$(whq path @ 2>/dev/null || true)"
if [[ -z "${repo_root:-}" ]]; then
  # Fallback: derive from the common git dir if possible
  if common_git_dir="$(git rev-parse --git-common-dir 2>/dev/null)"; then
    repo_root="${common_git_dir%/.git}"
  fi
fi

# If still unknown, skip any further actions
[[ -n "${repo_root:-}" ]] || exit 0

# --------- customize below ---------
# Files/directories to copy from the main worktree to the new worktree root
typeset -a FILES_TO_COPY
FILES_TO_COPY=(
  ".env"
  ".claude/settings.local.json"
)

# Commands to run in the new worktree (executed in order via eval)
typeset -a COMMANDS
COMMANDS=(
  "command -v pnpm   >/dev/null 2>&1 && pnpm install || true"
)
# -----------------------------------

# Copy files/directories if they exist
for f in "${FILES_TO_COPY[@]}"; do
  src="${repo_root%/}/$f"
  dest="${PWD%/}/$f"
  if [[ -e "$src" ]]; then
    mkdir -p -- "$(dirname -- "$dest")"
    cp -R -- "$src" "$dest"
  fi
done

# Run setup commands inside the new worktree
for c in "${COMMANDS[@]}"; do
  ( cd "$PWD" && eval "$c" )
done

exit 0
```
