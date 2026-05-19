# Spec ↔ Code Audit — 2026-05-18

Full cross-reference of `.claude/docs/spec/` against `internal/` and `cmd/dotd/`.
Items marked **VERIFY** need a human look before acting on them.

---

## High Severity

Real behavioural differences where the spec and code produce different outcomes.

---

### H1 — `compose: true` not parsed by code
**Spec:** compose.md §21 says mark a compose target with `compose: true` in `.dagger`.  
**Code:** `dagger.go` parses compose under `Composition.Enabled` (`yaml:"composition"` → `yaml:"enabled"`). The actual YAML the code accepts is:
```yaml
composition:
  enabled: true
```
`compose: true` at the top level is silently ignored (unknown field error with `KnownFields(true)` — or silently dropped if that isn't enforced).  
**Action:** Either update spec to show `composition:\n  enabled: true`, or add a `compose` YAML alias in `dagger.go`.

---

### H2 — `.dagger` YAML schema differs from dag.md §6
**Spec:** dag.md §6 shows a schema with top-level `dotd:` and `link:` sections (e.g. `dotd.when`, `link.link_root`).  
**Code:** All fields are flat at the top level — `when:`, `link_root:`, `defaults:`, `files:`, `actions:`, `composition:`, `conventions:`. No `dotd:` or `link:` nesting.  
**Action:** Rewrite dag.md §6 YAML examples to match actual flat schema. **VERIFY** — dag.md may have been partially updated already.

---

### H3 — `files:` dict format differs from dag.md §6
**Spec:** dag.md §6 shows `files:` as a **sequence** (list of objects with `path:` key).  
**Code:** `dagger.go` parses `files:` as a **map** (filename is the key). Actual format:
```yaml
files:
  dot-gitconfig-work:
    when: context=work
    actions: [link(~/.gitconfig)]
```
**Action:** Rewrite dag.md §6 `files:` examples to show map format.

---

### H4 — `@disable` annotation has no effect
**Spec:** dag.md §5 defines `@disable` to "exclude this file from all processing."  
**Code:** `@disable` is scanned as an annotation but never checked in walk, filter, or act. Files with `@disable` are processed normally.  
**Action:** Implement `@disable` in `walk.go` (skip node) or `filter.go` (exclude), or remove from spec.

---

### H5 — `dot-` transform only applies to first path component
**Spec:** overview.md §2 and architecture.md §16 say the `dot-` → `.` transformation applies uniformly to every path component.  
**Code:** `deriveLinkDest` in `act.go` uses `strings.SplitN(rel, sep, 2)` and only transforms `parts[0]`. A file at `conf/dot-config/dot-foo/bar` would produce `~/.config/dot-foo/bar`, not `~/.config/.foo/bar`.  
**Action:** Fix `deriveLinkDest` to iterate all path components and apply the transform to each.

---

### H6 — `nosync-` not stripped from derived symlink destinations
**Spec:** overview.md §2 implies `nosync-work/conf/dot-gitconfig → ~/.gitconfig`.  
**Code:** `deriveLinkDest` uses `filepath.Rel(n.LinkRootDir, n.Path)` — if `nosync-work/` is between the link root dir and the file, `nosync-work` appears in the destination path.  
**Action:** Strip `nosync-` prefix from each path component in `deriveLinkDest`, same as `node.DeriveName` does for logical names.

---

### H7 — `dotd-packages.yaml` manifest files never scanned
**Spec:** package-manifests.md §20 defines `dotd-packages.yaml` / `*.dotd-packages.yaml` as predicate-scoped package manifests scanned during `dotd package` commands.  
**Code:** `internal/manifest` package is implemented and tested, but **never called** from `package.go` or anywhere in the pipeline. `collectPackageRequests()` only scans `@require`/`@request` annotations from active nodes — manifest files are ignored entirely.  
**Action:** Wire `manifest.CollectFromPaths` into `collectPackageRequests()` in `package.go`.

---

### H8 — `nosync-` not stripped in compose output name derivation
**Spec:** compose.md §21 says output name derivation: strip `nosync-`, strip `dot-`, strip `.d`. Example: `nosync-dot-work.sh.d → work.sh`.  
**Code:** `act.go:107` only does `TrimPrefix(base, "dot-")` then `TrimSuffix(base, ".d")`. A dir named `nosync-dot-work.sh.d` produces `nosync-dot-work.sh` (TrimPrefix skips since it doesn't start with `dot-`).  
**Action:** Apply the same three-step derivation as `node.DeriveName` to compose output name computation.

---

### H9 — OS/distro/shell are not auto-detected
**Spec:** predicates.md §4 says `os`, `distro`, `shell` are auto-detected at runtime.  
**Code:** No built-in detection. `get-os` and `get-hostname` are hidden commands intended for use as `$(dotd get-os)` in `env.yaml` — detection only works if the user explicitly configures those shell expressions. Predicates like `@when os=macos` will produce `MissingKeyError` on a fresh install without env.yaml setup.  
**Action:** Either implement auto-detection in the env resolver, or update predicates.md to clarify that detection requires env.yaml configuration and document the `$(dotd get-os)` pattern.

---

## Medium Severity

Missing features or behavioural differences that don't cause silent wrong output.

---

### M1 — Compose target `name:` override not applied to directory nodes
**Spec:** compose.md §21 says `name:` in a compose target's `.dagger` overrides the output logical name.  
**Code:** `walk.go` always uses `node.DeriveName(rel)` for compose-target directory nodes; `cfg.Name` is never applied to them. (It is applied to `files:` dict entries correctly.)  
**Action:** Apply `cfg.Name` to compose-target nodes in the directory-emit path of `walk.go`.

---

### M2 — `dotd.when` doesn't gate traversal — it only filters
**Spec:** dag.md §6 says directory `when` "gates traversal of the entire subtree; if false, the directory is not entered at all."  
**Code:** Walk always enters all directories. The directory `when` is cascaded into child nodes' `effectiveWhen`, so they are filtered out by `Filter()` — but the directory is still traversed. No `fs.SkipDir` logic exists.  
**Action:** Minor optimization — behavioural output is the same, but spec description is stronger. Either add `SkipDir` or soften spec wording. Low priority.

---

### M3 — Missing keys always halt; no TTY-aware prompt
**Spec:** predicates.md §4 and architecture.md §16 say missing required keys prompt interactively on TTY, halt otherwise.  
**Code:** Always halts with a hint message (`"Hint: set it with --env ..."`). No TTY detection, no interactive prompt.  
**Action:** Implement TTY detection in `annotateKeyError` / apply path, or update spec to say "always halts with hint."

---

### M4 — `dotd compose apply` subcommand missing
**Spec:** compose.md §21 defines `dotd compose apply` as a standalone subcommand.  
**Code:** `compose_cmd.go` implements only `check` and `list`. `dotd apply` runs compose internally.  
**Action:** Add `dotd compose apply` subcommand, or remove it from spec.

---

### M5 — `@retain-prefix` annotation unimplemented
**Spec:** overview.md §2 and dag.md §5 define `@retain-prefix` to opt out of `dot-`/`nosync-` stripping.  
**Code:** Not handled anywhere — scanned as an annotation but ignored in all processing stages.  
**Action:** Implement or remove from spec.

---

### M6 — nosync gitignore warning not implemented
**Spec:** overview.md §2 and architecture.md §16 say `dotd setup` and `dotd check` warn if `nosync-*` is absent from `.gitignore`.  
**Code:** No `.gitignore` inspection anywhere.  
**Action:** Add `.gitignore` check to `dotd check`, or remove from spec.

---

### M7 — `config.yaml` / `env.yaml` split differs from spec
**Spec:** env.md §7 describes a single `env.yaml` containing both path config fields (`dotfiles_repo`, `link_root`, `bin_dir`, `generated_dir`, `init_file`) and env key overrides.  
**Code:** Two files — `config.yaml` stores path fields; `env.yaml` stores predicate env keys. `init_file` is not persisted in either file (resolved only via CLI arg / env var / default).  
**Action:** Rewrite env.md §7 to document the actual two-file split and the full field lists for each.

---

### M8 — `dotd setup` command referenced; code has `dotd init`
**Spec:** shell-init.md §8 and architecture.md §16 mention `dotd setup` behaviour (checking for source line in rc file, nosync gitignore warning).  
**Code:** The command is `dotd init`. `internal/setup` has `AppendSourceLine` and `HasSourceLine` but they are never called from `init_cmd.go`.  
**Action:** Either rename references in spec to `dotd init`, or implement the rc-file check in `init_cmd.go`.

---

### M9 — Conflicting `link(dest)` silently takes last value instead of erroring
**Spec:** actions.md §22 says "Duplicate `link(dest)` with conflicting destinations on the same node: hard error."  
**Code:** `mergeActions` (walk.go) updates the existing link's dest in-place when a second `link` is seen. `validateNode` checks `linkDests` but by then the list has only one entry (already merged). The error never fires.  
**Action:** Fix `mergeActions` to collect all link dests; fix `validateNode` to compare them and error on conflict.

---

## Low Severity

Naming mismatches, stale docs, cosmetic differences.

---

### L1 — `architecture.md` package list is stale
**Spec:** architecture.md §14 and §18 list packages `dag/`, `daggeryaml/`, `fileset/`, `linker/`, `initgen/`, `walk/`, `composer/` — none of which exist. The actual structure consolidates all pipeline stages into `internal/pipeline/`.  
**Action:** Rewrite §14 project structure and §18 status table to match actual package layout.

---

### L2 — Spec uses both `.dotd.yaml` and `.dagger`
**Spec:** Most spec sections (dag.md, symlinks.md, env.md, shell-init.md, cli.md) still say `.dotd.yaml`. index.md, actions.md, and compose.md use `.dagger`.  
**Code:** Uses `.dagger` exclusively (`ecosystem.ConfigFile`, walk.go skips both `.dagger` and `.dotd.yaml` defensively).  
**Action:** Global find-replace in all spec files: `.dotd.yaml` → `.dagger`.

---

### L3 — compose.md generated dir path is wrong
**Spec:** compose.md §21 says generated files go to `~/.config/dot-dagger/generated/`.  
**Code + env.md:** Both use `~/.local/share/dot-dagger/generated` (XDG data, not config).  
**Action:** Fix compose.md to say `~/.local/share/dot-dagger/generated`.

---

### L4 — `--verbose` flag absent; replaced by `--log-level`
**Spec:** cli.md §11 lists `--verbose` as a global flag.  
**Code:** Has `--log-level debug` and `--quiet` instead.  
**Action:** Update cli.md to document `--log-level` and `--quiet`; remove `--verbose`.

---

### L5 — `dotd bundle` and hidden `get-os`/`get-hostname` not in spec
**Code:** `bundle.go` implements `dotd bundle <path>` (concatenate node + transitive @after deps). `getters.go` has hidden `dotd get-os` and `dotd get-hostname`.  
**Spec:** Neither appears in cli.md.  
**Action:** Add `dotd bundle` to cli.md; document `get-os`/`get-hostname` in predicates.md or env.md as the intended mechanism for OS detection in env.yaml.

---

### L6 — "Conflict" symlink state reported as "not-a-symlink"
**Spec:** symlinks.md §10 defines the Conflict state (real file at dest — not a symlink).  
**Code:** runCheck reports this as `"not-a-symlink"`.  
**Action:** Align the error message to spec terminology.

---

## Not a gap (false positives to ignore)

- **Finding 18** (packages.yaml naming) — `packages.yaml` for registry, `dotd-packages.yaml` for manifests — consistent between spec and code.
- **Finding 21** (SourceLine uses `source`) — init.sh correctly uses POSIX `.`; the rc-file line using bash `source` is a different thing, intentional.
- **Finding 22** (symlink state naming) — absorbed into L6 above.
- **Finding 28** (convention dir nesting restriction) — moot now that convention dirs aren't magic in the code.
- **Finding 32** (shebang blank line) — code is more permissive, not less; no practical impact.

---

## Summary counts

| Severity | Count |
|----------|-------|
| High     | 9     |
| Medium   | 9     |
| Low      | 6     |

Highest-priority items to fix first: **H1** (compose marking broken), **H2/H3** (dag.md schema wrong — users can't write valid .dagger files by following the spec), **H4** (`@disable` silently no-ops), **H7** (manifest system dead code).
