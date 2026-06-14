# Your first machine

This tutorial walks through setting up dotd from scratch — creating a dotfiles repo, adopting your existing config files, and applying everything to a new machine.

## 1. Scaffold the repo

Run `dotd setup` to write the config files:

```sh
dotd setup
```

It asks for one thing:

- **Dotfiles directory** — where your dotfiles git repo lives (default: `~/dotfiles`)

That's the only stored setting. Every other path — `$bin`, `$config`, the generated-files and `init.sh` locations — resolves automatically from your environment (XDG). Run `dotd paths` to see exactly where they land on this machine.

Writes config.yaml and (if absent) env.yaml. Then run `dotd init` to scaffold the convention directories (`shellrc/`, `config/`, `bin/`) in your dotfiles repo and wire the init.sh source line into your shell:

```sh
dotd init
```

If you prefer to skip all prompts (both commands accept `-n`):

```sh
dotd setup --non-interactive
dotd init --non-interactive
```

## 2. Adopt your existing config files

Use `dotd adopt` to move existing files into the repo. It infers the right destination from the file's name and properties:

```sh
dotd adopt ~/.zshrc          # → config/dot-zshrc
dotd adopt ~/.gitconfig      # → config/dot-gitconfig
dotd adopt ~/.bashrc         # → config/dot-bashrc
```

Each adoption moves the file into your dotfiles repo and replaces the original with a symlink — `~/.zshrc` now points at `dotfiles/config/dot-zshrc`, so everything continues to work but is now managed.

You can override the destination with `--to`:

```sh
dotd adopt ~/.gitconfig --to config/dot-gitconfig-personal
```

## 3. Add annotations

Now that your files are in the repo, add annotations to declare conditions and dependencies. Open any file in `shellrc/` or `config/` and add annotations as comments at the very top:

```sh
# config/dot-zshrc
# @when shell=zsh
```

```sh
# shellrc/homebrew.sh
# @when os=macos AND exists(brew)
# @after shellrc/base/
```

Annotations are read at apply time — they have no effect at runtime. See [Annotations](../concepts/annotations.md) for the full reference.

## 4. Set your context

Open `env.yaml` in your dotfiles repo and set your `context` key. This is the main thing you control manually — most other keys (`os`, `shell`, `distro`) are detected automatically.

```yaml
env:
  context: personal
```

Run `dotd env show` to see the full resolved environment:

```sh
dotd env show
# os=macos
# shell=zsh
# distro=macos
# context=personal
```

## 5. Apply

```sh
dotd apply -f ~/dotfiles
```

This runs all four stages:

1. Resolves your environment
2. Installs any `@require`/`@request` packages
3. Creates symlinks for `config/` and `bin/` files
4. Writes `init.sh` with the active scripts in dependency order

Preview what would change without making any modifications:

```sh
dotd apply -f ~/dotfiles --dry-run
```

## 6. Wire up your shell

`dotd init` (step 1) already appended the `init.sh` source line to your shell RC
file. If you skipped `init`, add it once yourself:

```sh
# zsh
echo 'source ~/.local/share/dot-dagger/init.sh' >> ~/.zshrc

# bash
echo 'source ~/.local/share/dot-dagger/init.sh' >> ~/.bashrc
```

Reload your shell, and you're done.

## 7. Set up a second machine

On any new machine:

1. Clone your dotfiles repo
2. Install dotd (see [Installation](index.md))
3. Run `dotd apply -f ~/dotfiles`
4. Wire up your shell

dotd will evaluate conditions for the new machine — only files where `@when` is true get applied. Scripts written with `@when os=macos` stay quiet on Linux. Packages not available on this machine are skipped if they're `@request`, or produce an error if they're `@require`.

---

## What's next

- [Annotations](../concepts/annotations.md) — full explanation of what annotations do and how they work
- [Conditions](../concepts/conditions.md) — how `@when` expressions are written and evaluated
- [Load order](../concepts/load-order.md) — how `@after` dependencies control the order in `init.sh`
- [Reference: dotd](../reference/dotd.md) — all commands and flags
