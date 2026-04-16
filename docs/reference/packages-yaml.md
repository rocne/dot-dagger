# packages.yaml

Registry of packages and package managers. Lives at the root of your dotfiles repo. Defines what packages exist, how to install them, and which manager to prefer when multiple are available.

## Location

```
dotfiles/
  packages.yaml   ← must be at the repo root
```

## Format

```yaml
package_managers:
  priority: [brew, apt, dnf]   # global preference order

  brew:
    install:   brew install {package}
    uninstall: brew uninstall {package}
  apt:
    install:   apt install -y {package}
    uninstall: apt remove -y {package}
  dnf:
    install:   dnf install -y {package}
    uninstall: dnf remove -y {package}
  pip:
    install:   pip install {package}
    uninstall: pip uninstall -y {package}

packages:
  fzf:
    brew: {}
    apt: {}
```

## Defining packages

### Simple package (same name across all managers)

```yaml
packages:
  fzf:
    brew: {}
    apt: {}
    dnf: {}
```

### Binary name differs from package name

```yaml
packages:
  ripgrep:
    binary: rg   # dotd checks for `rg` on PATH, not `ripgrep`
    brew: {}
    apt: {}
    dnf: {}
```

### Custom install command for a specific manager

```yaml
packages:
  some-tool:
    brew:
      install: brew tap someorg/sometool && brew install some-tool
    apt: {}
```

### Custom existence check (no binary)

```yaml
packages:
  python-lib:
    check: "python3 -c 'import somelib'"
    pip:
      package: somelib   # pip install somelib (not pip install python-lib)
```

### Per-package manager preference

Override the global priority for a specific package:

```yaml
packages:
  ripgrep:
    prefer: [dnf, brew]   # prefer dnf for this package specifically
    binary: rg
    brew: {}
    dnf: {}
```

## Package manager selection

When a package needs to be installed, dotd selects the first manager from the preference list that:

1. Has an entry in the package's `managers` map
2. Is available on `$PATH`

The preference order is:
1. Package's own `prefer` list (if set)
2. Global `priority` list

## installable() vs installed()

These can be used in `@when` conditions:

```sh
# @when installed(ripgrep)    # true if binary is on PATH
# @when installable(ripgrep)  # true if a manager can install it
```

`installed()` uses the `binary` field from `packages.yaml` for name resolution. `installable()` checks whether any manager in the package's entry is available on `$PATH`.

## Detecting installed managers

`dotd setup` detects which package managers are installed on the current machine and pre-populates `packages.yaml` with entries for them.
