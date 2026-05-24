# CLI Command Groups Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Organize `dotd --help` output into labeled sections so users can orient quickly.

**Architecture:** Cobra v1.6+ supports `root.AddGroup(...)` + `cmd.GroupID` natively. All group assignments live in `newRootCmd()` in `main.go` — no changes to individual command constructors, no scattered group IDs.

**Tech Stack:** Go, `github.com/spf13/cobra` v1.10.2

---

## Part 1 — Proposed Grouping

### Current output (flat, alphabetical)

```
Available Commands:
  adopt        Move a file into the dotfiles repo and replace it with a symlink
  apply        Reconcile dotfiles: walk → filter → order → act → init.sh
  bundle       Bundle a node and its transitive @after dependencies into a single script
  check        Validate pipeline stages without writing anything
  completion   Generate shell completion script
  compose      Manage compose targets (assembled fragment files)
  config       Inspect and modify tool configuration
  dag          Inspect the dotfile dependency graph
  env          Inspect and modify env.yaml
  help         Help about any command
  init         Interactive wizard: create config.yaml and env.yaml
  list         List dotfile nodes and their status
  package      Package management — filtered by active predicates
```

### Proposed groups

| Group | Commands | Rationale |
|-------|----------|-----------|
| **Core Commands** | `apply`, `check`, `list`, `adopt` | Primary dotfiles workflow — run to act and inspect |
| **Configuration** | `init`, `env`, `config` | Manage tool and environment settings |
| **Advanced** | `dag`, `bundle`, `compose`, `package` | Power-user inspection and generation |
| _(ungrouped)_ | `completion` | Shell tooling — Cobra auto-places in "Additional Commands" |

### Target output

```
Core Commands:
  adopt        Move a file into the dotfiles repo and replace it with a symlink
  apply        Reconcile dotfiles: walk → filter → order → act → init.sh
  check        Validate pipeline stages without writing anything
  list         List dotfile nodes and their status

Configuration:
  config       Inspect and modify tool configuration
  env          Inspect and modify env.yaml
  init         Interactive wizard: create config.yaml and env.yaml

Advanced:
  bundle       Bundle a node and its transitive @after dependencies into a single script
  compose      Manage compose targets (assembled fragment files)
  dag          Inspect the dotfile dependency graph
  package      Package management — filtered by active predicates

Additional Commands:
  completion   Generate shell completion script
  help         Help about any command
```

---

## Part 2 — Challenges to the Grouping

### Challenge 1: `adopt` belongs in Configuration, not Core

**Argument:** You run `adopt` when building out your dotfiles repo — adding a new file to be managed. That's a setup action, not a daily pipeline action. The pipeline is `apply`/`check`/`list`.

**Counter:** `adopt` is the action you take every time you want to pull a new file under management — it's not one-time setup. Someone who frequently moves files into their dotfiles repo runs it as often as `apply`. "Core" means essential to the dotfiles workflow, not necessarily frequent.

**Verdict:** Keep `adopt` in Core. The workflow is: `adopt` a file, then `apply` to reconcile. They belong together.

---

### Challenge 2: `compose` and `package` are Core for users who use them

**Argument:** Someone who builds compose targets runs `compose list` / `compose check` as often as `apply`. Similarly, `package status` is a daily health-check for users who declare packages in their dotfiles.

**Counter:** Both features are opt-in. Most users never touch them. The grouping should optimize for the first-time reader scanning `--help`. Power users already know where their commands live after the first lookup.

**Verdict:** Keep `compose` and `package` in Advanced.

---

### Challenge 3: "Advanced" has a bad connotation — implies hard to use

**Argument:** `dag`, `bundle`, `compose`, `package` aren't *hard*, just less common. "Advanced" may discourage users from exploring them.

**Counter:** Alternative labels — "Tools", "Inspect", "Utilities" — all blur the signal. "Advanced" communicates "you don't need this on day one" clearly. Users don't avoid commands because of the label; they explore when they need to.

**Verdict:** Keep "Advanced". If this becomes a recurring complaint, revisit.

---

### Challenge 4: `env` and `config` are used frequently — not just setup

**Argument:** `env show`, `env get`, `env set` are inspection commands you run often when debugging predicate issues. `config show` too. Calling them "Configuration" buries them.

**Counter:** "Configuration" doesn't mean "infrequent" — it means "manages settings." The user who is debugging predicate issues will scan all groups anyway. The grouping helps first-time users, not power users who already have muscle memory.

**Verdict:** Keep `env` and `config` in Configuration.

---

### Challenge 5: `init` in Configuration is wrong — it's the first command you ever run

**Argument:** `init` creates config.yaml and env.yaml from scratch. It's not "modifying configuration" — it's bootstrapping. It deserves to be prominently placed, maybe first in Core.

**Counter:** `init` is a one-time wizard. It *creates* configuration files. Grouping it with `env` and `config` (which modify them) is semantically coherent. Putting it in Core would signal "run this often," which is wrong. First-time users will find it fine since they're reading all the docs anyway.

**Verdict:** Keep `init` in Configuration.

---

### Challenge 6: Cobra `--all` flag shows hidden commands — grouping disrupts this

**Argument:** Hidden commands (`get-os`, `get-hostname`) have no `GroupID`. When `--all` is passed and they become visible, they'll fall into "Additional Commands." Is that right?

**Counter:** Yes, that's fine. They're internal helpers — "Additional Commands" is appropriate. The `--all` flow is intentionally a power-user escape hatch.

**Verdict:** No action needed.

---

## Part 3 — Resolved Grouping

After challenge, the original grouping holds with no changes:

| Group ID | Title | Commands |
|----------|-------|----------|
| `core` | `Core Commands:` | `adopt`, `apply`, `check`, `list` |
| `config` | `Configuration:` | `config`, `env`, `init` |
| `advanced` | `Advanced:` | `bundle`, `compose`, `dag`, `package` |
| _(none)_ | _(Additional Commands)_ | `completion` |

---

## Part 4 — Implementation Plan

### File map

| File | Change |
|------|--------|
| `cmd/dotd/main.go` | Add `AddGroup` calls; assign `GroupID` on all subcommands after construction |

No changes to individual command constructor files — grouping is centralized in `newRootCmd()`.

---

### Task 1: Add groups and assign GroupIDs in `newRootCmd()`

**Files:**
- Modify: `cmd/dotd/main.go` (in `newRootCmd()`)

- [ ] **Step 1: Add group definitions after `root` is constructed**

In `newRootCmd()`, after `root := &cobra.Command{...}` and before the `AddCommand` block, add:

```go
root.AddGroup(
    &cobra.Group{ID: "core", Title: "Core Commands:"},
    &cobra.Group{ID: "config", Title: "Configuration:"},
    &cobra.Group{ID: "advanced", Title: "Advanced:"},
)
```

- [ ] **Step 2: Replace the existing `root.AddCommand(...)` block**

Replace the current block:

```go
root.AddCommand(
    getOSCmd,
    getHostnameCmd,
    newConfigCmd(),
    newInitCmd(cfg),
    newAdoptCmd(cfg),
    newApplyCmd(cfg),
    newCheckCmd(cfg),
    newEnvCmd(cfg),
    newListCmd(cfg),
    newBundleCmd(cfg),
    newPackageCmd(cfg),
    newComposeCmd(cfg),
    newDagCmd(cfg),
    newCompletionCmd(),
)
```

With this block that assigns GroupIDs before adding:

```go
// hidden internal helpers — no group
root.AddCommand(getOSCmd, getHostnameCmd)

// core
for _, cmd := range []*cobra.Command{
    newAdoptCmd(cfg),
    newApplyCmd(cfg),
    newCheckCmd(cfg),
    newListCmd(cfg),
} {
    cmd.GroupID = "core"
    root.AddCommand(cmd)
}

// config
for _, cmd := range []*cobra.Command{
    newConfigCmd(),
    newEnvCmd(cfg),
    newInitCmd(cfg),
} {
    cmd.GroupID = "config"
    root.AddCommand(cmd)
}

// advanced
for _, cmd := range []*cobra.Command{
    newBundleCmd(cfg),
    newComposeCmd(cfg),
    newDagCmd(cfg),
    newPackageCmd(cfg),
} {
    cmd.GroupID = "advanced"
    root.AddCommand(cmd)
}

// ungrouped (completion)
root.AddCommand(newCompletionCmd())
```

- [ ] **Step 3: Build and verify help output**

```bash
go build ./cmd/dotd && ./dotd --help
```

Expected: three labeled sections — "Core Commands:", "Configuration:", "Advanced:" — each with the correct commands. "Additional Commands:" at the bottom with `completion` and `help`.

- [ ] **Step 4: Verify `--all` still shows hidden commands**

```bash
./dotd --help --all
```

Expected: `get-os` and `get-hostname` appear (wherever Cobra places ungrouped unhidden commands — likely "Additional Commands").

- [ ] **Step 5: Verify subcommand help is unaffected**

```bash
./dotd apply --help
./dotd env --help
./dotd dag --help
```

Expected: each shows its own usage/flags/subcommands unchanged.

- [ ] **Step 6: Run tests**

```bash
go test ./cmd/dotd/...
```

Expected: all pass. No test currently asserts help output format, so no test changes expected.

- [ ] **Step 7: Commit**

```bash
git add cmd/dotd/main.go
git commit -m "feat: group dotd commands into Core, Configuration, and Advanced sections"
```

---

## Part 5 — Challenges to the Plan

### Challenge A: GroupIDs assigned via loop in `main.go` — not obvious which group a command is in without reading the block

**Argument:** A developer reading `env.go` has no idea what group `env` is in. The grouping context is invisible at the point of definition.

**Counter:** This is the right tradeoff. Group assignment is policy, not mechanics. It belongs with the registration code in `newRootCmd()`, not scattered in constructors. A grep for `"config"` in `main.go` is instant. If constructors owned GroupIDs, every constructor would need updating if we ever rename a group.

**Verdict:** Keep grouping in `main.go`.

---

### Challenge B: `newConfigCmd()` doesn't take `cfg` — inconsistent constructor signature

**Argument:** The loop assigns GroupIDs uniformly, but `newConfigCmd()` has a different signature from the others (no `*config` param). The loop still works, but the inconsistency is visible in the code.

**Counter:** `newConfigCmd()` is already inconsistent by necessity — it doesn't need `cfg` to resolve paths. That's a pre-existing fact unrelated to this change. The loop handles it fine since all constructors return `*cobra.Command`.

**Verdict:** No action. Don't fix unrelated inconsistency in this PR.

---

### Challenge C: No test asserts the help output — grouping could silently break

**Argument:** If a future refactor breaks `GroupID` assignment (e.g., someone moves a command to a different constructor pattern), tests won't catch it. Help output regression is invisible.

**Counter:** Writing a golden-file test for `--help` output is fragile — it would break every time we change a Short description. The integration test already runs the binary; if Cobra rejects invalid GroupIDs it panics at startup, which would be caught. Low risk.

**Verdict:** Skip the test. Accepted risk.

---

### Challenge D: `completion` ungrouped feels inconsistent — should it get its own group?

**Argument:** Having one command in "Additional Commands" alongside `help` looks odd. A "Shell:" or "Utilities:" group with just `completion` is cleaner.

**Counter:** Cobra's "Additional Commands" is a well-known pattern (e.g., `kubectl`). Adding a group for one command is more noise than signal. If we add more utility commands later, we can add the group then.

**Verdict:** Leave `completion` ungrouped.
