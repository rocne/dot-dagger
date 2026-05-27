# dotd

A dotfiles manager for people who use more than one machine.

If your setup is a single laptop running one OS and one shell, a handful of symlinks and a `.zshrc` is probably fine. But if you work across a personal Mac, a work Linux box, maybe a remote server — each with different shells, different package managers, different software installed — keeping one set of dotfiles that behaves correctly everywhere gets complicated fast.

The kind of problems dotd is designed for:

| | |
|---|---|
| **Multiple OS and shells** | Your Mac aliases and Homebrew paths don't belong on your Fedora box. Your zsh config and bash config share a lot but not everything. |
| **Work/personal coexistence** | Work aliases, internal hostnames, and credentials that can never go in a public repo — kept in a private or local-only directory, gated with `@when context=work`, coexisting cleanly with the rest of your dotfiles. |
| **Tool availability** | Scripts that use `fzf` or `ripgrep` shouldn't silently break on machines where those aren't installed. |
| **Package manager fragmentation** | Personal Mac uses Homebrew. Work Linux uses `apt` and you can't replace it. Remote server has `dnf`. One package name, right install command per machine. |
| **Script load order** | `00-base.zsh`, `10-path.zsh`, `20-tools.zsh` — hacking lexical sort into a load order. Breaks the moment you need to insert something, and says nothing about *why* the order matters. |

dotd's approach: **annotate files, not shell code**. Each file declares when it should be active and what it depends on. dotd evaluates those declarations once when you run `apply`, builds the active file set for this machine, and writes a clean `init.sh` with no runtime branches.

```sh
#!/bin/bash
# @when os=macos
# @after shellrc/base/
# @require ripgrep

alias ls='ls -G'
export HOMEBREW_PREFIX="/opt/homebrew"
```

The same annotation system drives symlink management and package installation. One mental model, one place to declare intent.

---

## How it works

dotd runs four stages in sequence:

1. **Env** — detects your environment (OS, distro, shell) and loads any overrides from `env.yaml`. This produces the resolved environment used for all condition evaluation.
2. **Fileset** — walks `shellrc/`, `conf/`, and `bin/`, evaluates `@when` conditions, and builds the active file set for this machine.
3. **Packages** — reads `@require` and `@request` annotations and installs packages using whichever manager is available.
4. **Symlinks + init.sh** — creates symlinks for `conf/` and `bin/` files; resolves `@after` dependencies and writes a single `init.sh` that sources only the active scripts in the right order.

Each stage is also available standalone: `dotd env`, `dotd dag`, `dotd link`, `dotd package`.

[Get started →](getting-started/index.md){ .md-button .md-button--primary }
[Annotation reference →](reference/annotations.md){ .md-button }
