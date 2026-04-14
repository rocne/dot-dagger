# dotp — Package Management

`dotp` manages package installation for the dotr suite. It registers the `@require` and `@request` annotations and the `installable()` / `installed()` predicate functions with `dota`.

---

## Annotations

### `@require pkg`

Hard requirement. The file is gated — only active if `installable(pkg) || installed(pkg)`.

- If installable but not installed: dotp installs it
- If not installable and not installed: error (loud failure in both standalone and orchestrated)

### `@request pkg`

Soft ask. The file is always active regardless of package state.

- If installable but not installed: dotp installs it
- If not installable and not installed: silently skip

The distinction in both modes is error behaviour. The gating (file exclusion from active set) is the orchestrated expression of `@require`'s loudness — rather than error, the file is excluded when the requirement cannot be met.

---

## Predicate Functions

### `installed(pkg)`

True if the package's binary is present on PATH. Uses the registry to resolve the binary name (e.g. `installed(ripgrep)` checks for `rg`). Falls back to the logical name if no binary override is defined.

### `installable(pkg)`

True if the registry has an entry for `pkg` with at least one package manager present in the current environment. No network call — purely a local registry + PATH check.

Both functions are registered with `dota` and usable in any `@when` expression.

---

## Package Registry (`packages.yaml`)

Lives at the dotfiles repo root. Defines logical package names, package manager entries, binary names, and install command overrides.

```yaml
# packages.yaml

package_managers:
  brew:
    install:   brew install {package}
    uninstall: brew uninstall {package}
    update:    brew upgrade {package}
  apt:
    install:   apt install -y {package}
    uninstall: apt remove -y {package}
    update:    apt upgrade -y {package}
  dnf:
    install:   dnf install -y {package}
    uninstall: dnf remove -y {package}
    update:    dnf upgrade -y {package}
  pacman:
    install:   pacman -S --noconfirm {package}
    uninstall: pacman -R {package}
    update:    pacman -Su --noconfirm {package}
  cargo:
    install:   cargo install {package}
    uninstall: cargo uninstall {package}
    update:    cargo install {package}
  pip:
    install:   pip install {package}
    uninstall: pip uninstall -y {package}
    update:    pip install --upgrade {package}

packages:
  fzf:
    brew: {}
    apt: {}

  ripgrep:
    binary: rg          # binary name differs from package name
    brew: {}
    apt: {}
    dnf: {}
    pacman: {}

  python-dateutil:
    pip:
      package: python-dateutil
    apt:
      package: python3-dateutil

  some-tool:
    brew:
      install: brew tap someorg/sometool && brew install some-tool
    apt: {}

  no-binary-lib:
    check: "python3 -c 'import somelib'"    # no binary — custom existence check
    pip:
      package: somelib
```

### Defaults

- Logical name = package name = binary name unless overridden
- `check` defaults to `which {binary}`
- Package manager commands are templates; `{package}` and `{binary}` are substituted at runtime

---

## Package Manager Priority

Declared in the `dote:` section of `.dotr.yaml` at the dotfiles repo root. dotp walks the priority list and uses the first manager that has an entry for the requested package and is present on PATH.

```yaml
# .dotr.yaml
dote:
  package_managers:
    priority: [brew, apt, dnf, pacman, pip, cargo]
```

A package only defined for `cargo` uses `cargo` even if `brew` is higher priority — priority only breaks ties when multiple managers have an entry.

---

## Standalone Mode

dotp walks the dotfiles tree, evaluates `@when` predicates (via `dota` + `dote`), and acts on `@require` / `@request` annotations for all active files.

Standalone behaviour is identical to orchestrated behaviour — the only difference is that standalone dotp builds its own `FileSet` rather than receiving one from `dotr`.

---

## CLI

| Command | Description |
|---------|-------------|
| `dotp install` | Install all packages for active files (default action) |
| `dotp check` | Report package status without installing |
| `dotp list` | List all packages declared across active files |

`--dry-run` prints what would be installed without running any package manager commands.

---

## `installable()` in `@when`

Since `installed()` and `installable()` are registered predicate functions, they are usable directly in `@when` expressions independent of `@require` / `@request`:

```bash
# @when installed(nvim)
# This file is only active if nvim is already installed — dotp won't install it
```

This allows predicate-only gating without triggering package installation.
