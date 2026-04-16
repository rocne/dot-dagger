# Spec Review

Working doc for reviewing and polishing the spec. Issues are addressed one by one; resolutions are noted before spec edits are made.

---

## Issues

### 1. `scripts/` vs `dots/` operation distinction not stated directly
**Location:** overview.md §2, dag.md, symlinks.md, architecture.md, index.md  
**Severity:** Medium  
**Issue:** The spec never directly states the key semantic difference: `scripts/` files are *sourced* into the shell; `dots/` files are *symlinked* into `$HOME`. A reader has to piece this together from scattered sections. More fundamentally, `dots/` is misleadingly named — it implies "dotfiles broadly" when the actual purpose is narrow: config files that a third-party tool expects at a fixed filesystem path. That's the only real reason symlinking is needed. Anything executable or sourceable doesn't need a symlink — it just needs to be in the DAG. The name also causes confusion because the `dot-` prefix convention (for filesystem visibility) is a separate concern that applies everywhere, not just inside `dots/`.  
**Status:** Resolved  
**Resolution:** Rename `dots/` → `conf/`. The purpose description in the table should make clear this is for config files that third-party tools expect at a specific path. The `dot-` prefix and `nosync-` prefix conventions are unchanged and apply everywhere. This is a broad rename — every occurrence of `dots/` in examples and prose across all spec files must be updated.

---

### 2. `@module` concept undefined
**Location:** dag.md §5 (annotations table), cli.md  
**Severity:** High  
**Issue:** `@module <n>` appears in the annotations table with purpose "Declare module membership (for organisational tooling)." But a module is never defined. `dotd module list` lists "directories with their active file counts" — so are modules directories? Can a file declare membership in a different module than its directory? `dotd add --module` references it but the relationship is opaque. The design decisions table in architecture.md never mentions modules.  
**Status:** Resolved  
**Resolution:** Drop modules entirely. Directories are the natural organisational unit — a `tmux/` directory with its `scripts/`, `bin/`, and `conf/` subdirectories is implicitly a module. Remove `@module` from the annotations table, remove `dotd module create` and `dotd module list` from the CLI, and remove the `--module` flag from `dotd add`. No replacement needed.

---

### 3. `dotd check` vs `dotd status config` overlap unexplained
**Location:** cli.md  
**Severity:** Medium  
**Issue:** `dotd check` is described as "Validate predicates, DAG, annotations, `.dot-dagger.yaml` files." `dotd status config` is "Config and annotation validation." These appear to overlap. If they differ (e.g. check is offline/static, status config reflects live resolved state), that distinction should be stated. If they're the same, one should be removed or made an alias.  
**Status:** Resolved  
**Resolution:** Drop `dotd status` and all its subcommands (`dotd status config`, `dotd status env`, `dotd status files`). Replace with a single `dotd check` command that covers everything — state inspection and error detection. Can be expanded into subcommands later if needed.

---

### 4. `dotd diff` vs `dotd apply --dry-run` relationship unstated
**Location:** cli.md  
**Severity:** Low  
**Issue:** Both `dotd diff` and `dotd apply --dry-run` exist. The spec doesn't explain the relationship. Are they identical? Is `diff` a shorthand alias? More fundamentally, the value of a diff is unclear — the only deployment artifacts are symlinks and `init.sh`, and `dotd check` already covers state inspection.  
**Status:** Resolved  
**Resolution:** Drop `dotd diff` entirely. Remove it from the CLI command table. `--dry-run` remains as a global flag on `dotd apply` for anyone who wants a preview, but there is no dedicated diff command.

---

### 5. `retain_prefix` on directory node semantics unclear
**Location:** dag.md §6, overview.md §2  
**Severity:** Medium  
**Issue:** The `.dot-dagger.yaml` example shows `retain_prefix: true` under `directory:`. But overview.md says: "The `RetainPrefix` flag applies only to the file's own path component; intermediate directory components are always transformed." What does retaining prefix mean for the directory node itself? If it means "don't transform this directory's name in the symlink path," that's a meaningful distinction and needs stating. If it means the same as the file-level flag, it contradicts the "intermediate components are always transformed" rule and shouldn't appear in the example.  
**Status:** Resolved  
**Resolution:** `retain_prefix` is uniform — it applies to any path component, file or directory, with no special cases. Remove the sentence "The `RetainPrefix` flag applies only to the file's own path component; intermediate directory components are always transformed" from overview.md. All components follow the same rule: `dot-` is transformed to `.` unless `retain_prefix` is set for that node.

---

### 6. Managed bin dir path: hardcoded or configurable?
**Location:** symlinks.md §9, env.md §7  
**Severity:** Low  
**Issue:** symlinks.md shows `~/.local/bin/dot-dagger/` as the managed bin dir destination. It's never stated whether this path is hardcoded or settable in `config.yaml`.  
**Status:** Resolved  
**Resolution:** The managed bin dir is configurable in `config.yaml`, defaulting to `~/.local/bin/dot-dagger/` (or the XDG bin dir if `$XDG_BIN_HOME` is set). Additionally, the convention directory names (`scripts/`, `bin/`, `conf/`) are also configurable in `config.yaml` as a power-user option — not prominently documented, but supported. Update symlinks.md and env.md to reflect this.

---

### 7. "repo root" is ambiguous
**Location:** env.md  
**Severity:** Low  
**Issue:** env.md says `config.yaml` lives at "repo root." This must mean the *dotfiles* repo root, not the dot-dagger tool repo, but a new user could reasonably wonder. Should say "dotfiles repo root" throughout.  
**Status:** Resolved  
**Resolution:** Replace "repo root" with "dotfiles repo root" throughout. Also note that the dotfiles repo location itself is configurable — `dotd` reads it from the env config (`env.yaml`), so users are not required to run `dotd` from within the dotfiles repo.

---

### 8. `dotd install` gitignore behavior missing from CLI docs
**Location:** cli.md, overview.md  
**Severity:** Low  
**Issue:** overview.md states that `dotd install` ensures `nosync-*` is in `.gitignore` before any other operation. The CLI table entry for `dotd install` in cli.md says only "Set up dot-dagger — rc wiring, first-run env prompts." The gitignore step is absent.  
**Status:** Resolved  
**Resolution:** Update the `dotd install` description in cli.md to mention the gitignore check. Also clarify in overview.md that `nosync-` functions primarily by being stripped by `dotd` at runtime — it is the user's responsibility to gitignore those files. `dotd install` (and `dotd check`) will warn and offer to add `nosync-*` to `.gitignore` if it's missing, but will not do so silently or as a hard requirement.

---

### 9. `✂` symbol not in output style table
**Location:** shell-init.md §12  
**Severity:** Low  
**Issue:** The output example shows `✂  dot-dagger` as a header line, but `✂` is not listed in the symbol table. It's decorative branding with no functional meaning.  
**Status:** Resolved  
**Resolution:** Remove the `✂  dot-dagger` header line from the output example. Branding can be revisited later.

---

### 10. `nosync-` symlink destination example missing
**Location:** overview.md §2 or symlinks.md §9  
**Severity:** Low  
**Issue:** overview.md states `nosync-` is stripped from both the logical name and the symlink destination, but there's no example showing the symlink destination stripping. The `dot-` examples are thorough; `nosync-` deserves the same.  
**Status:** Resolved  
**Resolution:** Add an example showing `nosync-` stripped from the implicit symlink destination (e.g. `nosync-work/conf/dot-gitconfig → ~/.gitconfig`). Also clarify that this stripping only applies to *implicit* destinations — if `@symlink` is declared explicitly, the destination is taken literally with no transformation. `@symlink` is the mechanism for overriding the default behavior.

---

### 11. No concrete "any depth" example in §2
**Location:** overview.md §2  
**Severity:** Low  
**Issue:** The spec says "conventions apply at any depth" but gives no example of a topic-grouped layout. A single concrete example (e.g. `tmux/scripts/`, `tmux/bin/`, `tmux/conf/`) would make this tangible for new users. Also, "any depth" turns out to be wrong.  
**Status:** Resolved  
**Resolution:** Replace "conventions apply at any depth" with the actual rule: special dirs are recognised anywhere in the tree as long as you haven't already passed through a special dir to get there. `scripts/conf/` and `scripts/scripts/` are ignored — once inside a special dir, no further special dirs are recognised. No hard depth cap. Add a concrete example showing both root-level and topic-grouped layouts (including a `nosync-` case).

Also introduces `link_root` — a `.dot-dagger.yaml` `directory:` field that overrides the base path for symlink destinations in that subtree (default `~`). `@symlink` destinations follow standard path rules: absolute if starting with `/` or `~/`, otherwise implicitly relative to `link_root`. Flag for further review before spec is finalised.
