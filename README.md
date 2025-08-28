# whq

Minimal helper around `git worktree` for a single Git repository. It creates and manages per-branch worktrees in a predictable, ghq-like layout outside of `.git`.

- Operates only inside a Git repository; otherwise exits with: `whq: not inside a Git repository`.
- Worktrees live under `WHQ_ROOT/<host>/<owner>/<project>/<worktree>` (default `WHQ_ROOT=~/whq`).
- The main worktree is detected from `git worktree list --porcelain` and referred to as `@`.

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

## Behavior

- `repo_root`: First `worktree` entry from `git worktree list --porcelain`.
- `WHQ_ROOT`: From env var; defaults to `~/whq` (leading `~` expanded). The repository’s
  worktree root is `WHQ_ROOT/<host>/<owner>/<project>` derived from the `origin` remote
  (SSH, HTTPS, or scp-like). A trailing `.git` is stripped.
- All Git operations are executed via subprocesses; Git’s stderr is surfaced on failures.

## Commands

- `whq add <branch>`: Create `repo_whq_root/<branch>` worktree; creates branch from HEAD if missing.
- `whq path <branch|@>`: Print absolute path to a worktree, or `repo_root` for `@`.
- `whq list [-p]` / `whq ls [-p]`:
  - Default: `@` for main worktree, others relative to `repo_whq_root` when possible.
  - `-p`: Print absolute paths for all worktrees.
- `whq rm [-f|--force] [-b|--branch] <branch>`: Remove a worktree; with `-b`, also delete the local branch (`-d`, fallback to `-D`).
- `whq prune`: Run `git worktree prune`.
- `whq root`: Print `repo_whq_root`.

## Notes

- The CLI does not change your shell’s directory. Use `whq path` with shell substitution to `cd`.
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

