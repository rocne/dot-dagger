# Duplicated Code and Logic

Logic implemented more than once where the copies can (and in places already do) diverge, with no shared abstraction.

### [AUDIT-013] `.dagger` action strings parsed two divergent ways (dir vs file)

**Original ID:** C-001
**Location:** `internal/pipeline/walk.go:305-324` (`parseDaggerActions`, callsite `:131`), `internal/pipeline/walk.go:406-419` (`parseActionString`, callsites `:281,433,448`)
**Severity:** High
**Description:** The same `.dagger` `actions:` field is parsed two ways. Directory-level actions use `parseDaggerActions`, which only recognizes `compose`/`source`/`nosource`/`link(...)` via `HasPrefix("link(")`/`HasSuffix(")")` and silently drops anything else (no fallback Action). File-level actions use `parseActionString`, a generic `IndexByte('(')`/`LastIndexByte(')')` split that returns a real Action for any `type(dest)` plus a malformed-paren fallback.
**Justification:** The two parsers genuinely diverge: `parseActionString` is strictly more capable. A future/unknown action type declared at directory level is discarded, while the identical string at file level produces an Action.
**Impact:** The behavior of `actions:` silently depends on whether it is declared on a directory or file `.dagger`. Adding any new action type requires editing two parsers; forgetting `parseDaggerActions` (buried in an `else if` chain) means dir-level support silently no-ops.
**Cross-reference:** AUDIT-006, AUDIT-017.

### [AUDIT-014] Link-destination conflict detection done two ways with different scope

**Original ID:** C-004
**Location:** `internal/pipeline/actions.go:75-80` (`validateNode`, intra-node, unresolved `a.Dest`), `internal/pipeline/act.go:127-130` & `:155-158` (`Act`, cross-node, resolved `dest`)
**Severity:** Medium
**Description:** "Two things must not claim the same link destination" is decided in two unrelated places with different scope and messages. `validateNode` rejects conflicts within a single node comparing raw `a.Dest` strings at validate time; `Act` rejects conflicts across nodes comparing fully `resolveLink`-ed paths at act time. Because `validateNode` compares unresolved dests, two link actions with `~/.x` and `/home/u/.x` pass validation but resolve to the same path; an empty-dest link bypasses `validateNode`'s check entirely.
**Justification:** A change to what counts as a "conflict" (e.g. normalizing `~` before comparing) must be made in both, or validate and act disagree about which configs are legal.
**Impact:** A config can pass `dotd check`'s validation yet fail in `apply` — the conflict definition lives in two divergent places.

### [AUDIT-015] Link-conflict + nosource pre-scan blocks duplicated inside `Act`

**Original ID:** C-005
**Location:** `internal/pipeline/act.go:108-113` & `:140-145` (nosource pre-scan), `internal/pipeline/act.go:125-134` & `:153-159` (link-conflict)
**Severity:** Medium
**Description:** Within `Act`, the compose-node branch and the regular-node branch each contain a byte-identical "scan for `ActionNoSource`" loop and a byte-identical link-conflict block (differing only in `Src`: `genPath` vs `n.Path`).
**Justification:** Both copies are identical; a fix to conflict reporting or nosource semantics must be applied in both branches of the same function.
**Impact:** Easy to patch one branch and miss the other, leaving the compose and regular paths with inconsistent conflict/nosource handling.
**Cross-reference:** AUDIT-014.

### [AUDIT-016] Read-command preamble (resolveEnv → Walk → filterWithPrompt → Order) copied across six commands

**Original ID:** C-009
**Location:** `cmd/dotd/dag_cmd.go:24-39`, `cmd/dotd/list_cmd.go:47-65`, `cmd/dotd/bundle.go:44-62`, `cmd/dotd/compose_cmd.go:33-42` & `:62-74`, `cmd/dotd/package.go:31-39` (& `:73-81`,`:102-110`), `cmd/dotd/main.go:283-305` (`runPipeline`, write-path analogue)
**Severity:** Medium
**Description:** Every read command repeats the same four-step opening: `resolveEnv(cfg)` → annotate key error → `pipeline.Walk(cfg.files)` → `filterWithPrompt(nodes, resolved, isTTYStdin())` → `pipeline.Order(active)`. The error-wrap strings have already drifted (`"walk: %w"` in dag_cmd.go:30 vs `"walk %s: %w"` with `cfg.files` in list_cmd.go:54 / bundle.go:51) — concrete copy-paste divergence.
**Justification:** Already-inconsistent error messages prove the copy-paste. A `func (cfg *config) walkOrdered() ([]RawNode, error)` is the natural missing abstraction (`runPipeline` is the existing write-path analogue).
**Impact:** A change to the shared read path (add a validate step, change TTY plumbing) must touch 6+ sites; error messages are already inconsistent.
**Cross-reference:** AUDIT-030.

### [AUDIT-017] Third `key(args)` paren-splitter; behavior matches but logic triplicated

**Original ID:** C-002
**Location:** `internal/annotation/annotation.go:77-91` (`parseKeyArgs`), `internal/pipeline/walk.go:406-419` (`parseActionString`)
**Severity:** Medium
**Description:** The "split `name(args)` on first `(` / last `)`" pattern exists in two near-identical implementations: `parseKeyArgs` and `parseActionString` use the byte-for-byte same `IndexByte('(')`/`LastIndexByte(')')`/TrimSpace algorithm with the same malformed-paren fallback, differing only in return type. (The third site, `parseDaggerActions`, does not actually split — see AUDIT-013 — so it is tracked there, not double-listed here.)
**Justification:** A shared `splitParen(s) (head, body string, ok bool)` would back both; as-is the annotation and action layers can drift.
**Impact:** A fix to paren handling (nested parens, escaped chars) must be applied in two places.
**Cross-reference:** AUDIT-013.

### [AUDIT-018] Inline `term.IsTerminal(os.Stdin.Fd())` in adopt instead of `isTTYStdin()` helper

**Original ID:** C-003 (merged: D-005, F-005)
**Location:** `cmd/dotd/adopt.go:92`, `cmd/dotd/filter_prompt.go:95-97` (`isTTYStdin`)
**Severity:** Low
**Description:** `adopt.go:92` writes `yes || !term.IsTerminal(os.Stdin.Fd())` inline — the exact body of `isTTYStdin()` — while every other TTY check in the package routes through that helper (passed as the `isTTY` arg to `filterWithPrompt`). adopt is the lone holdout, forcing it to import `charmbracelet/x/term` directly. (init/setup use a third approach: per-read EOF fallbacks.)
**Justification:** Same single line viewed three ways — as duplication (C-003), an ownership reach-around (D-005), and a UX-consistency hazard (F-005). It is the canonical-resolution anti-pattern in miniature: a named owner exists but the raw primitive is used.
**Impact:** If TTY detection ever needs adjustment (honor `--no-input`/`$CI`, check stdout too), adopt's prompt gate silently won't follow. No user-visible bug today.
**Cross-reference:** AUDIT-022.

### [AUDIT-019] "Does this node have a compose action?" decided three ways

**Original ID:** C-006
**Location:** `cmd/dotd/compose_cmd.go:117-124` (`hasComposeAction`), `internal/pipeline/act.go:82-88` (inline `hasCompose`), `internal/pipeline/actions.go:57-66` (`seenCompose` in `validateNode`)
**Severity:** Low
**Description:** The predicate "node has an `ActionCompose`" is implemented as a standalone command-layer helper and re-implemented inline at least twice in the pipeline layer. There is no exported `pipeline` method (e.g. `RawNode.HasCompose()`) all three could share, though `compose_cmd.go` clearly wanted one.
**Justification:** Same trivial decision in three layers; a richer compose model (compose-with-args) would need all three updated. (Note: Low — debatable whether worth a shared method.)
**Impact:** Low; logic is trivial and stable, but a compose-model change touches three sites.

### [AUDIT-020] Tilde-expansion logic duplicated across layers

**Original ID:** C-007
**Location:** `cmd/dotd/init_cmd.go:174-182` (`expandTildeStr`), `internal/pipeline/act.go:226-243` (`expandDest`)
**Severity:** Low
**Description:** The `~` / `~/` home-expansion branch is identical in both (`if path == "~"` → home; `HasPrefix("~/")` → `filepath.Join(home, path[2:])`); `expandDest` additionally handles `~bin`/`~bin/`. They live in different packages so cannot trivially share.
**Justification:** The home-tilde rule is now defined twice; if `~user` or empty-`$HOME` handling is added, setup/init would expand a path differently from how the pipeline resolves the link destination.
**Impact:** Cross-package duplication that can diverge under future tilde-handling changes; consolidation is non-trivial.

### [AUDIT-021] `composeGenName` is a zero-value wrapper

**Original ID:** C-008
**Location:** `cmd/dotd/compose_cmd.go:126-128`
**Severity:** Low
**Description:** `composeGenName(n)` is a one-line pass-through `return pipeline.ComposeFileName(n.Path)` with a single caller (`:49`), adding indirection but no abstraction value.
**Justification:** Pure inlining opportunity; it hides the `.Path` access but provides nothing else.
**Impact:** Trivial; just indirection. Could inline `pipeline.ComposeFileName(n.Path)` at the callsite.
