# Code-Smell Audit Findings — 2026-06-13

STATUS: REMEDIATION COMPLETE — PRs #111–#117 merged. Only D6 + P4 leftovers open.
- B1, B2, B3 → PR #111 (v0.4.3). Bonus root-cause fix: Act read no fragment
  content in dry-run → compose check was perma-stale, masked by exit-0.
- S1, S2 → PR #112.
- B5, B6, B7, B8 → PR #113 (B7 spec-confirmed via docs/concepts/conditions.md).
- O1, O2, O3, O4, S7, O6 → PR #114 (channel policy ratified: mutation results
  → stdout, never gated by --quiet; ui.Hintf replaces dead ui.Tipf).
- D7, D9, S8, S9, S10, B9, C6 + D8 (parseFlags, Catalog, EmptyRegistry)
  → PR #115. D8's adopter.DryRun was evaluated and KEPT — tested library
  capability, not dead.
- B4/D1, D2, D3, D4, D5, S3, S4, S5, S6, C4 → PR #116 (final planned PR;
  merged `693b6bd`, ~v0.4.8).
- Deep internal/ pass (user-requested) completed 2026-06-13 — added B5–B9,
  S8–S10, D7–D11, C5, C6.
- REMAINING (all P4, fix-when-touched): O5 skip-vocabulary/indent, C2 alias
  policy, C3 prefix stragglers, C5 unknown-@after debug log, D10 packages
  manager-loop consolidation, D11 files:-dict link-conflict alignment.
- X DECISIONS → PR #117 (merged `aef4956`, v0.4.9): C1 `dag check`→`dag order`
  (`check` alias kept), teardown override scope (now honors resolved paths),
  P5 `unapply --all` shadow (root `--all` deleted; `help --all` owns it).
  REMAINING X: D6 prompt-stack consolidation — investigated, feasible on huh,
  deferred as a dedicated PR.
Baseline: audit at `ae944d8`; deep pass at `5d629c9`. Evidence + method:
`audit-2026-06-13-trail.md`. Action plan: `audit-2026-06-13-plan.md`.

Categories: **B** behavior bug (found via consistency lens) · **S** single-source
violation · **O** output-channel/styling · **D** duplication/generalizability ·
**C** convention/naming · **X** discussion items (need a user decision).

Priority: P1 fix now (user-visible or unsafe) · P2 should fix ·
P3 worthwhile cleanup · P4 judgment call / low stakes.

---

## B — Behavior bugs

### B1 (P1) — `teardown` advertises `--dry-run` but ignores it
`main.go:53-56` lists `dotd teardown` as a `--dry-run` owner (so it shows in
`teardown --help`), but `runTeardown` never reads `cfg.dryRun`.
`dotd teardown --yes --dry-run` **deletes config.yaml and env.yaml**.
Fix: honor `cfg.dryRun` in teardown (print preview, stop before confirm/execute,
mirroring unapply_cmd.go:125). Add test.

### B2 (P1) — `adopt` auto-accepts on non-TTY stdin
`adopt.go:92`: `nonInteractive := yes || !isTTY(cmd.InOrStdin())` — piped/CI
stdin silently **proceeds with a file move** without confirmation.
Directly violates the convention stated at prompts.go:7 ("Never auto-accept a
destructive or filesystem-mutating action on EOF"). unapply/teardown cancel in
the same situation (promptConfirm EOF → false).
Fix: non-TTY without `--yes` should refuse with a hint ("pass --yes for
non-interactive use") or cancel safely. Check integration tests for reliance
on the current behavior before changing.

### B3 (P1) — `compose check` promises a non-zero exit and never delivers
compose_cmd.go:87 Long: "Exits non-zero if any target is missing or stale."
RunE returns nil in every branch (`:139-144`); `--json` path too. The
`compose check && echo "all clean"` example is therefore wrong.
No compose_cmd_test.go exists.
Fix: return an error (silenced-usage style like `check: issues found`) when
`hasStale`; add tests for text + json + clean paths.

### B4 (P2) — validation set differs between write and read pipelines
`runPipeline` validates **all walked nodes before filtering** (main.go:463);
`walkOrdered` validates **only active nodes after filter/order** (main.go:520),
while its own comment (:498) claims validation is shared "so a config that
apply/check rejects also fails under list/dag/bundle/compose/package".
A node that is invalid but filtered out fails `apply` yet passes `list`.
Fix: extract one shared preamble (see D1) and pick one semantic — recommend
validate-all-nodes (matches apply, catches latent config errors early).

### B5 (P1) — compose-dir `when:` skips paren-wrapping → AND/OR precedence bug
Every when-expression source is paren-wrapped before AND-joining (annotation
CombineWhen wraps each `@when`; applyDefaults wraps `defaults.when`
walk.go:378; `files:` entries wrap walk.go:290) — **except** the compose-target
dir's own `when:` at walk.go:150: `combineWhen(state.when, cfg.When)` passes
it raw. The parser gives AND precedence over OR (ast.go grammar), so a compose
dir with `when: "os=macos OR os=linux"` under an inherited `(context=work)`
becomes `(context=work) AND os=macos OR os=linux` =
`(context=work AND os=macos) OR os=linux` — active on any linux machine
regardless of context. Fix: wrap non-empty `cfg.When` in parens at :150 (or
teach combineWhen to wrap operands); add a regression test with OR.

### B6 (P2) — annotate wizard validates @when without the canonical parser
`WhenType.Validate` (annotation/registry.go:50-58) hand-checks for a `=`.
Consequences: rejects valid predicates with no condition (`exists(tmux)`,
`installed(fzf)`); accepts invalid ones that contain `=` (`os=a AND`).
`predicate.Parse` is the canonical validator and annotation→predicate is an
acyclic import. Fix: `Validate = predicate.Parse(s) err`. (Keep the friendly
hint text by wrapping the parse error.)

### B7 (P2) — package registry never wired into predicate evaluation
`Evaluator.RegisterPackageRegistry` (builtins.go:26) has zero production
callers. During filter, `installable(x)` is always false (empty registry) and
`installed(x)` ignores packages.yaml `binary:` aliases — while
`dotd package check` consults the loaded registry. The same fact evaluates
differently in `@when` vs `package check`. SPEC CHECK DONE:
docs/concepts/conditions.md:102-103 explicitly promises registry-backed
resolution for both functions — current behavior contradicts shipped docs.
Fix: load the registry in the pipeline preamble (cheap; LoadFile returns an
empty registry when absent) and wire via RegisterPackageRegistry; build the
Evaluator once per Filter call instead of per node while there.

### B8 (P3 — verify first) — `files:`-dict nodes drop walk fields
walk.go:304-313 builds RawNode for `files:` entries without `LinkRootDir`,
`IsCompose`, `ComposeTarget` (annotation-path nodes set all three,
walk.go:243-255). `deriveLinkDest` returns "" when `LinkRootDir` is empty
(act.go:193), so a `files:` entry relying on an inherited `link_root` for a
derived destination silently produces no link; a `files:` entry inside a
compose dir isn't treated as a fragment. Write a reproducing test before
fixing — severity unconfirmed.

### B9 (P4) — per-package install override skips `{package}` substitution
`InstallCmd` (packages.go:253) returns `me.Install` raw; the global-template
path substitutes `PlaceholderToken` (:264). An override containing
`{package}` ships literally. Substitute on both paths (idempotent for
overrides without the token).

---

## S — Single-source violations

### S1 (P1) — tool name half-hardcoded in generated env.yaml template
setup_cmd.go:120:
`fmt.Sprintf("os: $(dotd get-os)\nhostname: $(%s get-hostname)\n", ecosystem.ToolD)`
— first line literal `dotd`, second uses the constant, in the same string.
Fix: both via `ecosystem.ToolD`. Adopt the rule: **prose/help text may write
`dotd` literally (readability); anything written to disk or executed must use
`ecosystem.ToolD`.** Record the rule in audit-guide.md.

### S2 (P1) — stale `conf/` convention name shipped in `--help`
Canonical name is `adopter.DirConfig = "config"` (renamed long ago), but:
- adopt.go:31-33 (inference table in Long), :41, :43 (examples) say `conf/`
- annotate_cmd.go:29 example: `dotd annotate conf/dot-gitconfig`
- adopter/adopter.go:38 + Infer rule docs: `conf/dot-bashrc`, `<conf>/`
PR #107 fixed this in README/docs site but missed help strings and Go doc
comments — the binary still tells users a directory name that does not work.
Fix: sweep `conf/` → `config/` in all help text and comments. Acceptance:
`grep -rn '\bconf/' cmd/ internal/ --include='*.go'` → 0 hits.

### S3 (P3) — two names for the env.yaml default path
`env.DefaultPath` (env.go:144) is a pure forwarder to
`ecosystem.DefaultEnvFile`. main.go:333 and teardown_cmd.go:69 call the
forwarder; every sibling default in resolvePaths calls `ecosystem.Default*`
directly. Two access paths to one canonical value.
Fix: delete the forwarder, use `ecosystem.DefaultEnvFile` everywhere (keeps
all Default* in one package). Mechanical.

### S4 (P2) — pluralization not unified after PR #109
`plural` lives in config_cmd.go:49 (a command file — odd owner) and is used by
main.go/env.go. Hand-rolled `%d symlink(s)` remains at unapply_cmd.go:113,115
and teardown_cmd.go:53.
Fix: move `plural` to a neutral home (e.g. cmd/dotd/format.go or internal/ui)
and use it at the three remaining sites.

### S5 (P3) — trivial wrappers used inconsistently
- `envYamlPath(cfg)` (env.go:55) — returns `cfg.envFile`; used at 2 of 5
  sites in the same file (set/edit), while show/diff read `cfg.envFile` raw.
- `loadConfig(path)` (config_cmd.go:31) — literally `dotcfg.Load(path)`.
Fix: delete both wrappers; call the underlying value/function everywhere.
A wrapper that not everyone uses is worse than no wrapper.

### S6 (P3) — `"~bin"` magic prefix has no constant
act.go owns the expansion (`:220,223`, doc `:212-214`) and init_cmd.go:161
writes the literal into the scaffolded bin/.dagger template. ActOptions.BinDir
comment also restates it.
Fix: `pipeline.BinPrefix = "~bin"` const; use in act.go and init template.

### S7 (P2) — hint rendering has two owners with different formats
Root handler main.go:107 prints `hint:  %s` (two spaces, aligns with
`error:`); filter_prompt.go:63 prints `hint: ...` (one space) for the persist
suggestion. Neither goes through a ui helper; ui has Tipf but no Hintf.
Fix: add `ui.Hintf` (or reuse Tipf — decide one label) and route both sites
through it. Root handler styling folds into O2.

### S8 (P2) — RC source-line header duplicated with two encodings
setup/shell.go: AppendSourceLine writes the literal `# dotd — generated shell
init` (:96); RemoveSourceLine matches `"# dotd \xe2\x80\x94 generated shell
init"` (:114) — same string, escaped bytes. Append/remove must stay in sync
and nothing enforces it. Fix: one `const sourceLineHeader` used by both.

### S9 (P3) — predicate syntax help table duplicated
`WhenType.Description` (annotation/registry.go:38-44) and `conceptsText`
(concepts_cmd.go PREDICATES section) carry the same syntax table; they have
already drifted in column spacing. Fix: export one table (e.g.
`predicate.SyntaxHelp` const) and reference it from both.

### S10 (P4) — generated-file headers brand three ways
"# Generated by dot-dagger — do not edit by hand." (initgen.go:27),
"# Bundled by dotd — do not edit by hand." (bundle.go:77),
"# Generated by dotd — run with: sudo sh" (packages.go:307). Pick one identity
(suggest `ecosystem.Name`) and a shared helper/const. Also: `package generate`
help says "Pipe to sh to execute" while the script header says "run with:
sudo sh" — align the privilege story.

---

## O — Output channel / styling

### O1 (P2) — mutation results split across two channels
The "I did the thing" line:
- `apply` → `cfg.log.Infof` (links/init.sh summaries, main.go:626,633)
- `adopt` → `cfg.log.Infof` ("adopted …", adopt.go:128-132)
- `unapply`/`teardown` → `ui.OKf(stdout)` ("removed …")
- `setup`/`init` → `ui.OKf(stdout)` ("wrote …")
Consequence: `--quiet` hides apply/adopt results but not unapply/teardown/
setup/init results. Same message class, two behaviors.
Fix: decide a policy and write it into audit-guide.md. Recommendation:
**mutation results are user-facing status → stdout via ui helpers; cfg.log is
for diagnostics (debug/warn) only.** That matches the majority (4 of 6
commands) and the log.go package comment ("Data output … via cobra"). Moving
apply/adopt summaries to stdout changes test expectations — scope carefully.

### O2 (P2) — root error handler bypasses ui styling
main.go:101-107 prints raw uncolored `cancelled` / `error:` / `hint:` while
every in-command error/warning is colored via ui. Errors — the most important
output — are the only unstyled channel.
Fix: `ui.Errf(os.Stderr, …)` + the S7 hint helper in main(). (Raw os.Stderr
is correct here — this is the one place outside cobra's writers.)

### O3 (P3) — `compose check` mixes channels in one path
Per-file status via `ui.Missingf/Wrongf(errOut)` (compose_cmd.go:134-136) but
the summary via `cfg.log.Warn/Infof` (:140,142). One report, two channels —
`--quiet` produces orphaned detail lines with no summary.
Fix: pick one channel for the whole report per the O1 policy.

### O4 (P3) — three cancellation styles
- prompt abort sentinel → `fmt.Fprintln(os.Stderr, "cancelled")` (main.go:101)
- promptConfirm → `ui.Skipf(out, "cancelled")` to stdout (prompts.go:206)
- adopt decline → `cfg.log.Info("adopt cancelled")` (adopt.go:101)
Fix: one rendering (suggest ui.Skipf "cancelled" to stderr — status, not data)
used by all three paths.

### O5 (P4) — skip/none vocabulary and indentation drift
"skip: %s" (teardown) vs "  skipping" (init) vs "  exists %s" (setup) vs
"nothing to remove" (unapply); setup/init indent status lines two spaces,
teardown/unapply don't. Judgment: unify verbs ("skip …", "exists …") and drop
or standardize the indent when touching these files anyway; not worth a
dedicated PR.

### O6 (P3) — predicate Warn mode is dead code with a channel violation inside
eval.go: `NewEvaluator` always builds Strict (:104); `SetWarnOutput` has no
production caller; the Warn branch ui.Warnf's to a defaulted `os.Stderr`
(:49,85) — an internal package importing ui and writing terminal output.
Fix: delete Mode/Warn/SetWarnOutput and the ui import (keep Strict behavior
inline). If Warn mode is ever needed, reintroduce with caller-supplied writer.
Check spec (predicates.md) first — if Warn mode is specced, wire it properly
instead.

---

## D — Duplication / generalizability

### D1 (P2) — runPipeline / walkOrdered duplicated preamble
Both do guard → resolveEnv → Walk → filterWithPrompt → Order (+ validate at
different points — see B4). walkOrdered also discards the `disabled` list that
runPipeline debug-logs.
Fix: one `walkActive(cmd, cfg) (ordered, counts, err)` preamble; runPipeline
= preamble + Act. Fixes B4 and the disabled-logging asymmetry in one move.

### D2 (P3) — JSON/text branches duplicate computation
- env diff: filter logic written twice (env.go:260-273 / :280-292)
- package list: kind derivation twice (package.go:143-149 / :155-159)
- package check: installed loop twice (:62-69 / :71-78)
compose list/check, config show, dag check, list already do entries-first.
Fix: standardize the shape everywhere: **build []entry once → render (json |
text)**. Mechanical; reuses existing entry structs.

### D3 (P3) — configKeyArgs / envKeyArgs near-identical twins
config_cmd.go:37-47 vs env.go:43-53 — same body, different hint suffix.
Fix: one `keyArgs(nArgs int, usage, hint string) cobra.PositionalArgs` in
errors.go; both call it.

### D4 (P4) — repeated flag registrations
`--json` ×8 (identical desc "output JSON array" — verified accurate at all 8:
each emits a JSON array), `--yes` ×3, `--non-interactive` ×2.
Judgment: 8 one-liners are tolerable; a `addJSONFlag(cmd) *bool` helper makes
the desc single-source and is cheap. Do it opportunistically in D2's PR; not
worth its own.

### D5 (P3) — three file-existence idioms
`fileutil.Exists` (canonical, teardown/unapply) vs `os.IsNotExist(err)` after
Stat (init_cmd.go:51 — also the legacy errors API) vs
`errors.Is(err, fs.ErrNotExist)` (setup_cmd.go:116, main.go:650,668).
Fix: use `fileutil.Exists` where only existence matters; `errors.Is` where the
error itself is needed. Eliminate `os.IsNotExist` (superseded API).

### D6 (P4 / X) — two prompt stacks
huh forms (annotate, adopt, filter prompts) vs raw bufio prompts (setup, init,
unapply/teardown confirm). Documented in prompts.go header, and the bufio path
exists partly because tests drive prompts via piped stdin lines. Consolidating
on huh-accessible-mode everywhere is a real UX-uniformity win but touches
setup/init test driving patterns — spec-level change, brainstorm first.
No action this round beyond recording the trade-off.

### D7 (P3) — three atomic-write implementations; act.go writes non-atomically
fileutil.SaveYAML (temp+rename), initgen.writeAtomic (temp+rename),
annotation/write.go (`path+".tmp"`+rename, preserves mode). Meanwhile Act
writes compose-generated files with plain os.WriteFile (act.go:124) — the only
non-atomic write of generated output. Fix: `fileutil.WriteAtomic(path string,
data []byte, mode os.FileMode)`; SaveYAML, initgen, annotation.Write, and
act.go all use it.

### D8 (P3) — dead code
- `env.parseFlags` (env.go:109-122): zero callers; duplicates main.go's
  `--env` parsing with *different* syntax (comma-split vs repeatable flag).
- `packages.Catalog` (catalog.go, 103 lines): zero production callers —
  speculative wizard material. Check spec before deleting.
- `adopter.AdoptOptions.DryRun`: cmd-level runAdopt returns on cfg.dryRun
  before calling Adopt, so the in-Adopt dry-run path is unreachable from the
  CLI (verify no other callers, then remove one of the two layers).
- `predicate.emptyRegistry` (builtins.go) duplicates the empty-registry value
  packages.LoadFile returns for missing files — export
  `packages.EmptyRegistry()` or reuse LoadFile semantics.
(O6's dead Warn mode also lives in this bucket — already in PR 3 scope.)

### D9 (P4) — two shell-quoting owners
initgen.singleQuote (always quotes) vs cmd bundle.go shellQuote
(conditionally quotes). One `fileutil`/shared helper.

### D10 (P4) — packages manager-loop and dedupe duplication
`Installable` and `resolveInstallCmd` iterate ManagerOrder with identical
has-entry + lookPath logic; GenerateScript's `seen` dedupe duplicates cmd
`uniquePackages`. Consolidate opportunistically.

### D11 (P4) — `files:` dict drops conflicting links silently
walk.go:295-302 dedupes actions by Type, discarding a second `link(...)` with
a different dest; the annotation path keeps both so validateNode can report
the conflict (walk.go:469-481). Same input shape, different conflict
behavior. Align with the annotation path.

---

## C — Convention / naming

### C1 (X) — `check` means three different things
`dotd check` validates and exits non-zero; `compose check` validates (B3);
`dag check` only prints order — it checks nothing (`dag show`/`list` would be
honest). `package check` reports status, exits 0 regardless (its Long makes no
exit-code promise, so merely worth documenting).
Rename = breaking command change → user decision. Same bucket as deferred P5
(`unapply --all` shadow).

### C2 (P4) — `unapply` is the only aliased command (`remove`)
Either aliases are a feature (add the obvious ones elsewhere) or noise (drop
this one). Tiny; decide when touching unapply.

### C3 (P4) — error-wrap prefix convention is mixed
Command-name prefixes (`setup:`, `adopt:`, `unapply:`, `teardown:`,
`annotate:`) vs package prefixes (`env:`, `predicate:`, `packages:`,
`fileutil:`) vs bare (`walk %s:`, `order:`). Rule is implicitly "who am I" at
each layer and mostly holds; `walk`/`order`/`act` wraps in main.go are
stage-name prefixes — fine. Action: one paragraph in audit-guide.md defining
the rule; fix stragglers only when touched.

### C4 (P3) — banner switch duplicated in setup and init
setup_cmd.go:65-78 four-way switch and init_cmd.go:61-65 two-way both render
`ui.Header("dotd <cmd>") — <subtitle>`. Generalizable:
`bannerf(out, cmd, subtitle string)` deriving the name from
`cmd.CommandPath()` (kills the hardcoded command-name strings too).

### C6 (P3) — repo metadata files walk as nodes
Walk skips `ecosystem.ConfigFile`/`LegacyConfigFile` (walk.go:180) but not
`packages.yaml` (`ecosystem.PackagesFileName`) or a repo-root `env.yaml` —
they appear in `dotd list` as action-less nodes (observed in PR 5 smoke).
Harmless until a root `defaults.actions` exists, then they get sourced/linked.
Fix: skip `PackagesFileName` at the repo root in Walk (decide whether a
repo-root env.yaml deserves the same; it's a documented repo-layout file in
the e2e fixture). → PR 6.

### C5 (P4) — unknown `@after` references are silently ignored
order.go:42 `continue`s on an @after name with no matching node — a typo'd
dependency silently stops ordering. At minimum debug-log it; consider a warn
in `dotd check`.

---

## X — Discussion items (user decision needed)

1. **C1 rename** (`dag check` → `dag show`?) — breaking.
2. **D6 prompt-stack consolidation** — spec-level, affects test driving.
3. **O1 channel policy ratification** — recommendation above; changes
   apply/adopt output visible to users and tests.
4. **teardown scope rule** — teardown deliberately ignores `--config`/
   `--env-file` overrides (commented) but uses resolved `cfg.initFile`/
   `cfg.linkRoot` for RC detection. Half-and-half. Either honor resolved
   paths everywhere or document why RC detection is different.
5. **P5 carry-over** from 2026-06-12 audit: `unapply --all` shadows root
   `--all`. Still deferred.

---

## Tally

| Priority | Items |
|----------|-------|
| P1 | ~~B1, B2, B3~~ (PR #111), S1, S2, **B5** |
| P2 | B4, S4, S7, O1, O2, D1, **B6, B7, S8** |
| P3 | S3, S5, S6, O3, O4, O6, D2, D3, D5, C4, **B8, S9, D7, D8** |
| P4 | O5, C2, C3, D4, **B9, S10, D9, D10, D11, C5** |
| X  | C1, D6, O1-policy, teardown-scope, P5 |

Deep-pass coverage note: internal/ now read line-by-line (pipeline, annotation,
node, dagger, config, packages, predicate, setup, env, adopter, fileutil, log,
ui). Remaining unread: lexer.go internals (grammar doc reviewed), concepts
text body, test files (out of scope).
