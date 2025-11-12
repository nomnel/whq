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

- Create the default `.whq.json` template for post-add automation:
  - `whq init` (use `--force` to overwrite an existing file). The generated
    `copy`/`commands` arrays are empty so `whq add` keeps its no-op behavior
    until you customize them.
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

- `whq init [--force]`: Generate a `.whq.json` template in the repository root.
  - Default template keeps both `copy` and `commands` empty arrays so existing
    workflows remain unaffected until edited.
  - When the file already exists, the command aborts without overwriting unless
    `--force` is specified (stderr explains what was skipped).
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
- `whq version`: Print the CLI version string (current default `v0.0.4`,
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

## Post-add automation (`.whq.json`)

`whq add` natively supports repository-specific setup steps driven by a JSON
file in the repo root. Run `whq init` once to scaffold the template (add
`--force` to regenerate). The scaffold keeps both arrays empty so `whq add`
still behaves exactly like prior versions until you populate the lists. When
`.whq.json` is missing the behavior is identical to previous releases (silent
no-op).

### Schema

```json
{
  "post_add": {
    "copy": [
      ".env.example",
      "config/local/",
      "scripts/setup.sh"
    ],
    "commands": [
      "pnpm install",
      "mise run bootstrap"
    ]
  }
}
```

- `copy`: relative paths (files or directories) resolved from the repo root.
  Each entry is copied into the new worktree using the same relative path. The
  destination is overwritten if it already exists.
- `commands`: shell snippets executed via `bash -lc` inside the new worktree.
  Commands run sequentially and inherit the parent stdout/stderr streams.

### Execution & output

- `.whq.json` is read only from the repository root; no env overrides or parent
  lookups.
- Post-add processing runs immediately after `git worktree add` succeeds and
  before `Created worktree: <path>` is printed.
- Copy steps run before commands. Any failure aborts the remaining steps and
  makes `whq add` fail.
- Sample log lines:

```text
Post-add (.whq.json): starting (copy=2, commands=1)
Post-add copy: .env.example
Post-add cmd 1/1: pnpm install
Post-add (.whq.json): completed
```

- If automation succeeds the final confirmation line still prints. On failure,
  no summary is printed and the error is surfaced to stderr.
- When post-add fails, `whq` automatically calls the equivalent of
  `whq rm -b <branch>` to delete the just-created worktree. The branch is
  deleted only when it was created by this `whq add` invocation; existing
  branches are preserved while their failed worktrees are removed.

### Validation checklist

1. Create `.whq.json` in your repo root using the schema above.
2. Run `whq add demo-post-add`.
3. Confirm the copied files exist under the new worktree and that the expected
   command side effects (e.g., generated files) are present.
4. Review the stdout log to ensure the sequence matches the sample output.
