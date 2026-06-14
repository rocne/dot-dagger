# Code-Smell Audit Trail — 2026-06-13

Scope: consistency, single ownership, canonical sources, common output interface,
duplication, special cases, parameter/styling uniformity. Per user instruction:
**judgment applied** — items are evaluated, not blanket-flagged; deliberate
exceptions with justifying comments are recorded as evaluated-OK.

Companion docs:
- `audit-2026-06-13-findings.md` — every item, categorized + prioritized
- `audit-2026-06-13-plan.md` — PR-grouped action plan

Baseline at audit time: `main` = `ae944d8`, `go build ./...` clean,
`go test ./...` all packages ok (cached).

## Method

Followed `.claude/docs/audit-guide.md` passes 1–5, plus new lenses this session:
- flag-registration uniformity across commands
- JSON/text dual-render shape comparison
- prompt-stack and confirmation-default comparison across mutating commands
- advertised-flag vs honored-flag cross-check (`pathFlagOwners` vs RunE bodies)
- error-wrap prefix convention survey (`fmt.Errorf` corpus, uniq -c)

### Grep passes run

```
# Pass 1 — magic values, env reads, GOOS/home
grep -rn '"source"|"link"|"compose"|"no-source"|"generate"' internal/ cmd/
grep -rn '"\.dagger"|"\.dotd\.yaml"|"config\.yaml"|"env\.yaml"' internal/ cmd/
grep -rn 'ecosystem\.Default[A-Z]' cmd/dotd/
grep -rn 'os\.Getenv' cmd/dotd/ internal/
grep -rn 'runtime\.GOOS|os\.UserHomeDir' cmd/ internal/

# Pass 2 — duplication sites
grep -rn 'ExpandHome|os\.Stat|os\.Lstat|MkdirAll|yaml\.Marshal|SaveYAML|WriteFile' ...
grep -rn '(s)' cmd/ internal/           # hand-rolled pluralization
grep -rn 'func plural|plural(' ...
grep -rn 'hintError|Hint()' ...

# Pass 3/4/5 — output routing & ui helpers
grep -rn 'fmt\.Fprintf(os\.Stdout|fmt\.Println|fmt\.Printf|os\.Stderr' ...
grep -rn 'ui\.<every sprint + *f helper>' cmd/ internal/

# New lenses
grep -rn '\.Flags()\.|PersistentFlags()\.' cmd/dotd/       # flag uniformity
grep -rn 'json\.NewEncoder|MarshalIndent' cmd/dotd/        # JSON sites
grep -rn 'dryRun|dry-run' <mutating commands>              # advertised vs honored
grep -rn 'SetWarnOutput|predicate\.Warn|NewFuncRegistry|NewEvaluator' ...  # dead mode
grep -rn 'DefaultEnvFile|DefaultPath' ...                  # duplicate default chains
grep -rn 'fmt\.Errorf("' ... | sed/uniq                    # error prefix survey
grep -rn '"~bin' ...                                       # magic prefix constant
grep -rn '"dotd|ToolD' ...                                 # tool-name literal survey
```

### Files read in full

cmd/dotd: main.go, errors.go, prompts.go, setup_cmd.go, init_cmd.go,
teardown_cmd.go, unapply_cmd.go, adopt.go, env.go, config_cmd.go,
compose_cmd.go, list_cmd.go, dag_cmd.go, package.go, bundle.go,
annotate_cmd.go, filter_prompt.go, getters.go, concepts_cmd.go (head).

internal: ui/ui.go, ecosystem/ecosystem.go, fileutil/fileutil.go, log/log.go,
env/env.go (head), adopter/adopter.go (head), predicate/eval.go (Call/registry
section). Pipeline internals (walk/act/order/filter) checked via targeted greps
only — no fmt output, no canonical-source hits there.

## Key evidence (line-anchored)

- `cmd/dotd/main.go:53-56` lists `dotd teardown` as a `--dry-run` owner;
  `teardown_cmd.go` contains zero `dryRun` references → B1.
- `cmd/dotd/adopt.go:92` `nonInteractive := yes || !isTTY(cmd.InOrStdin())`
  vs `prompts.go:7` "Never auto-accept a destructive or filesystem-mutating
  action on EOF" → B2.
- `cmd/dotd/compose_cmd.go:87` Long promises non-zero exit; RunE returns nil
  on stale (`:139-144`) → B3. No compose_cmd_test.go exists.
- `cmd/dotd/main.go:463` validates **all** nodes before filter;
  `main.go:520` validates **active ordered** nodes after filter; comment at
  `:498-499` claims shared semantics → B4/D1.
- `cmd/dotd/setup_cmd.go:120` mixes literal `dotd` and `ecosystem.ToolD` in
  one Sprintf → S1.
- `adopter.DirConfig = "config"` (adopter.go:22) vs `conf/` in adopt.go help
  (:31-33,41,43), annotate_cmd.go:29 example, adopter.go doc comments
  (:38, Infer rules) → S2.
- `internal/env/env.go:144-146` DefaultPath forwards ecosystem.DefaultEnvFile;
  main.go:333 + teardown_cmd.go:69 use the forwarder, all sibling defaults use
  `ecosystem.Default*` → S3.
- `plural` defined `config_cmd.go:49`, used in main.go/env.go; hand-rolled
  `(s)` at unapply_cmd.go:113,115 and teardown_cmd.go:53 → S4.
- `envYamlPath` (env.go:55) used 2 of 5 sites; `loadConfig` (config_cmd.go:31)
  is a no-op wrapper → S5.
- `"~bin"` literal: init_cmd.go:161 template + act.go:16,220,223 — no shared
  constant → S6.
- hint rendering: main.go:107 `"hint:  %s"` vs filter_prompt.go:63
  `"hint: ..."` (different spacing, both raw fmt) → S7.
- Mutation-status channels: apply (main.go:626,633) + adopt (adopt.go:128-132)
  via `cfg.log.Infof` (hidden by --quiet); unapply/teardown/setup/init via
  `ui.OKf` to stdout → O1.
- Root handler main.go:101-107 raw uncolored `error:`/`hint:` while ui.Errf
  exists → O2.
- compose check mixes `ui.Missingf(errOut)` (:134-136) with `cfg.log.Warn`
  (:140) in one path → O3.
- Cancel styles: main.go:101 "cancelled"(stderr) / prompts.go:206
  ui.Skipf "cancelled"(stdout) / adopt.go:101 cfg.log.Info("adopt cancelled") → O4.
- Skip vocabulary + indent drift: teardown "skip: %s", init "  skipping",
  setup "  exists %s", unapply "nothing to remove" → O5.
- predicate Warn mode dead: NewEvaluator always Strict (eval.go:104);
  SetWarnOutput has no production callers; eval.go imports ui + defaults
  os.Stderr (:49) → O6.
- JSON/text dual-compute duplication: env.go diff (:260-273 vs :280-292),
  package.go list (:143-149 vs :155-159), check (:62-69 vs :71-78) → D2.
- configKeyArgs (config_cmd.go:37-47) ≅ envKeyArgs (env.go:43-53) → D3.
- `--json` registered 8×, desc "output JSON array"; `--yes` 3× → D4.
- Existence idioms ×3: fileutil.Exists / os.IsNotExist (init_cmd.go:51) /
  errors.Is(fs.ErrNotExist) (setup_cmd.go:116, main.go:650,668) → D5.
- Two prompt stacks: huh (annotate/adopt/filter) vs raw bufio
  (setup/init/unapply/teardown confirm) — documented in prompts.go header → D6.
- `dag check` prints order, checks nothing; `dotd check`/`compose check`
  validate → C1. unapply sole alias "remove" → C2. Error-prefix survey shows
  mixed command-name vs package-name prefixes → C3. setup 4-way banner switch
  (setup_cmd.go:65-78) + init 2-way (init_cmd.go:61-65) → C4.

## Evaluated and accepted (no action)

- teardown's direct `ecosystem.DefaultConfigFile()`/`env.DefaultPath()` —
  commented deliberate exception (removes system files regardless of flag
  overrides). Noted: it half-uses resolved cfg (initFile, linkRoot) for RC
  detection; see findings "discussion" section.
- `os.UserHomeDir()` in setup_cmd.go:52 — commented: prompt-typed `~`
  expansion only, not config resolution.
- `os.Getenv("EDITOR")` in launchEditor — universal convention exception.
- `runtime.GOOS`/`os.Hostname` in getters.go — these ARE the canonical detectors.
- `annotation.ActionType.Options()` string duplication — paired sync comments
  added by PR #109; import cycle prevents sharing.
- `c.Stdout = os.Stdout` in launchEditor — documented subprocess TTY exception.
- Data output via plain `fmt.Fprintf(cmd.OutOrStdout())` in env/config/list/
  dag/package — machine-parseable by design.
- `pathFlagOwners` manual map — acceptable centralization, but B1 shows it can
  drift from behavior; plan adds a teardown fix + test.
- ui.go color aliases (Conflict==Wrong etc.) — semantic naming layer, intentional.
- init_cmd template strings containing `source`/`link` — file-content data,
  not program logic; building them from pipeline constants judged overkill.
