# dotr

A dotfiles management suite for people who use more than one machine.

If your setup is a single laptop running one OS and one shell, a handful of symlinks and a `.zshrc` is probably fine. But if you work across a personal Mac, a work Linux box, maybe a remote server — each with different shells, different package managers, different software installed — keeping one set of dotfiles that behaves correctly everywhere gets complicated fast.

The kind of problems dotr is designed for:

| | |
|---|---|
| **Multiple OS and shells** | Your Mac aliases and Homebrew paths don't belong on your Fedora box. Your zsh config and bash config share a lot but not everything. |
| **Work/personal coexistence** | Work aliases, internal hostnames, and credentials that can never go in a public repo — kept in a private or local-only directory, gated with `@when context=work`, coexisting cleanly with the rest of your dotfiles. |
| **Tool availability** | Scripts that use `fzf` or `ripgrep` shouldn't silently break on machines where those aren't installed. |
| **Package manager fragmentation** | Personal Mac uses Homebrew. Work Linux uses `apt` and you can't replace it. Remote server has `dnf`. One package name, right install command per machine. |
| **Script load order** | `00-base.zsh`, `10-path.zsh`, `20-tools.zsh` — hacking lexical sort into a load order. Breaks the moment you need to insert something, and says nothing about *why* the order matters. |

dotr's approach: **annotate files, not shell code**. Each file declares when it should be active and what it depends on. dotr evaluates those declarations once when you run `apply`, builds the active file set for this machine, and writes a clean `init.sh` with no runtime branches.

```sh
#!/bin/bash
# @when os=macos
# @after scripts/base/
# @require ripgrep

alias ls='ls -G'
export HOMEBREW_PREFIX="/opt/homebrew"
```

The same annotation system drives symlink management and package installation. One mental model, one place to declare intent.

---

## How it works

dotr is made up of four tools that each own one stage of the process:

1. **`dote`** detects your environment — OS, distro, shell — and loads any overrides from `env.yaml`. This produces the resolved environment used for all condition evaluation.
2. **`dotd`** walks `scripts/`, evaluates `@when` conditions, and resolves `@after` dependencies into a load order. It writes a single `init.sh` that sources only the active scripts in the right order.
3. **`dotl`** walks `conf/` and `bin/`, plans symlinks into `$HOME` and `$PATH`, and applies them.
4. **`dotp`** reads `@require` and `@request` annotations and installs packages using whichever manager is available.

`dotr` runs all four in a single pass.

[Get started →](getting-started/index.md){ .md-button .md-button--primary }
[Annotation reference →](reference/annotations.md){ .md-button }
