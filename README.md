# dot-dagger

A dotfiles composition engine. `dot-dagger` walks your dotfiles directory, evaluates per-file conditions based on your environment, and produces a single shell init file and set of symlinks — with no manual configuration required.

## What it does

- **Predicate-gated files** — files are included or excluded based on your OS, distro, shell, or whether a command exists. No more `if [ "$(uname)" = "Darwin" ]` scattered through your shell config.
- **DAG-ordered sourcing** — declare ordering relationships between files; `dot-dagger` resolves them into a deterministic `init.sh` you source once from any shell's rc file.
- **Symlink management** — dotfiles and bin commands are symlinked into place automatically, with drift detection to catch anything that's gotten out of sync.
- **Convention over config** — `scripts/`, `bin/`, and `dots/` directories just work. Annotation or YAML config is only needed when you want something non-default.

## CLI

```sh
dotd install    # resolve, generate init.sh, and apply symlinks
dotd check      # validate config and report all issues
dotd apply      # apply symlinks only
dotd status     # show drift between repo and home
```

## Status

Early development — nothing is implemented yet. Design is complete.

## License

TBD
