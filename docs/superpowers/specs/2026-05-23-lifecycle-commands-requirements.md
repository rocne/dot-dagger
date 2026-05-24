# Lifecycle Commands — Requirements

## Context

`dotd init` currently does too much: writes system-level tool config (config.yaml, env.yaml) AND scaffolds directory-level convention files (.dagger). There is no way to undo any of it.

The goal is a clean two-tier lifecycle:

| Tier | Set up | Tear down |
|------|--------|-----------|
| System (tool config) | `setup` | `teardown` |
| Reconcile (symlinks, init.sh) | `apply` | `unapply` |
| Directory scaffold | `init` | _(manual — it's the user's repo)_ |

Breaking changes to `dotd init` are acceptable.

---

## Global philosophy: atomicity

No command leaves the system in a half-changed state. Before touching anything, validate that all steps can complete. If validation fails, abort without making any changes. This applies to all destructive commands (`teardown`, `unapply`).

---

## R1 — `dotd setup`

### What it does
- Fully interactive wizard that configures the tool at the system level
- Writes config.yaml to the platform config dir
- Writes env.yaml only if it does not already exist
- If config.yaml already exists: loads current values and shows them as per-field defaults — user presses Enter to keep or types new value
- If config.yaml does not exist: shows system-derived defaults, prompts per-field

### What it does NOT do
- Does not scaffold `.dagger` files (that is `dotd init`)
- Does not create symlinks or init.sh (that is `dotd apply`)
- Does not touch the shell RC file (future work — see WIP below)

### Preconditions
- None — can run on a fresh machine

### Relative path handling
- Any relative path entered (e.g. `.`) is resolved to absolute before saving

### Output
- config.yaml written
- env.yaml written (if absent)
- Prints next steps: run `dotd init`, add files, run `dotd apply`

### WIP / deferred
- Shell RC file wiring (source line) — design incomplete; setup does NOT touch RC file yet

---

## R2 — `dotd init`

### What it does
- Scaffolds `.dagger` convention files in the configured dotfiles repo
- Always interactive — no `--yes`, no positional args
- Prompts for directory roles (one prompt per role):
  - "shell scripts directory" → writes `.dagger` with `defaults.actions: [source]`
  - "config files directory" → writes `.dagger` with `defaults.actions: [link]`
  - Empty input skips that role
- For each named directory: creates it if absent, writes `.dagger` if absent
- Idempotent: skips existing `.dagger` files

### What it does NOT do
- Does not write config.yaml or env.yaml (that is `dotd setup`)
- Does not create symlinks (that is `dotd apply`)

### Preconditions
- config.yaml must exist. If absent, exits immediately:
  `"no config found — run 'dotd setup' first"`

### Output
- One `.dagger` file per named directory entered
- Prints what was written
- Prints next steps: add files, run `dotd apply`

---

## R3 — `dotd teardown`

### What it does
- Removes config.yaml from the platform config dir
- Removes env.yaml from the platform config dir
- Strips the dotd source line and its comment header from the shell RC file
- Prunes the config dir if empty after removal

### Atomicity
- Validates all steps before touching anything
- If any removal would fail (permissions, unexpected state), aborts without making any changes

### Pre-action checks
- Attempts a pipeline walk to detect whether symlinks are still active; also checks for `.dagger` files
- If the walk fails (env.yaml absent, dotfiles repo missing, etc.), the check is skipped silently — not a fatal error
- If active symlinks or `.dagger` files are detected: prints a warning ("symlinks still active — consider running `dotd unapply` first") then continues to the confirmation prompt — does not hard-block

### Confirmation
- Shows full preview of what will be removed
- Prompts `[y/N]` — default No
- `--yes` / `-y`: skip prompt

### What it does NOT do
- Does not remove symlinks or init.sh (that is `dotd unapply`)
- Does not remove `.dagger` files
- Does not remove the dotfiles repo

### If files already absent
- Reports "not found, skip" per item and continues — not an error
- If env.yaml is absent, shell cannot be determined → RC stripping is skipped (reported as "skipped — env.yaml absent")

### Output
- Warning if lingering symlinks or `.dagger` files detected
- Preview of what will be removed
- Per-item removal report
- On cancellation: "cancelled", exits 0

---

## R4 — `dotd unapply`

### What it does
- Re-runs the pipeline to determine the expected link plan (Src → Dest pairs)
- For each planned link: if a symlink at Dest exists AND its target equals Src, queues it for removal
- Queues init.sh for removal if present
- Shows preview, prompts for confirmation, then removes all queued items

### Atomicity
- Collects the full removal list before touching anything
- All removals happen after confirmation; if any fail, reports the error (partial failure possible post-confirmation — pre-confirmation is fully validated)

### `--all` flag
- Skips predicate filtering — walks all nodes regardless of `@when`
- Removes any symlink pointing into the dotfiles repo
- Use case: user applied with `--env context=work` and wants full cleanup without re-specifying flags

### What it does NOT do
- Does not remove symlinks that don't point into the dotfiles repo
- Does not remove orphaned symlinks from nodes no longer in the walk (deferred)
- Does not remove `.dagger` files or config files

### Preconditions
- config.yaml and env.yaml must exist
- Dotfiles repo must exist — hard error if not found, no fallback

### Flags
- `--dry-run` (global): preview only, no changes
- `--all`: skip predicate filtering
- `--yes` / `-y`: skip confirmation prompt

### Output
- Preview: "Will remove N symlink(s) [and init.sh]:"
- If nothing to remove: "nothing to remove", exits 0
- On `--dry-run`: preview only, exits 0
- Per-removal report
- On cancellation: "cancelled", exits 0

---

## Ordering constraint

```
dotd unapply    # needs config.yaml + env.yaml to walk and find links
dotd teardown   # removes config.yaml and env.yaml last
```

Each command is independent. Partial teardown is valid. `teardown` warns if run before `unapply`.

---

## UX consistency — destructive commands

| | `teardown` | `unapply` |
|--|--|--|
| Preview before acting | yes | yes |
| `[y/N]` prompt | yes | yes |
| `--yes` to skip | yes | yes |
| `--dry-run` | no | yes (global flag) |
| Atomicity | full pre-validation | collect then execute |
| Exit 0 on cancel | yes | yes |

---

## Deferred

- **Orphaned symlink detection:** scan linkRoot for symlinks pointing into the dotfiles repo that aren't in the current walk output; report as warnings from Walk so all commands get it for free.
