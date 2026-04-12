# Package Manager Design

**Status:** WIP — extracted from main spec for standalone tool development

---

## Package Managers Section

Enumerates supported package managers with default commands. dot-dagger ships with defaults for common ones. Users can override defaults or add custom package managers.

```yaml
package_managers:
  brew:
    install:   brew install {package}
    uninstall: brew uninstall {package}
    update:    brew upgrade {package}
    check:     which {binary}

  apt:
    install:   apt install -y {package}
    uninstall: apt remove -y {package}
    update:    apt upgrade -y {package}
    check:     which {binary}

  dnf:
    install:   dnf install -y {package}
    uninstall: dnf remove -y {package}
    update:    dnf upgrade -y {package}
    check:     which {binary}

  pacman:
    install:   pacman -S --noconfirm {package}
    uninstall: pacman -R {package}
    update:    pacman -Su --noconfirm {package}
    check:     which {binary}

  pip:
    install:   pip install {package}
    uninstall: pip uninstall -y {package}
    update:    pip install --upgrade {package}
    check:     which {binary}

  cargo:
    install:   cargo install {package}
    uninstall: cargo uninstall {package}
    update:    cargo install {package}
    check:     which {binary}
```

---

## Packages Section

Maps logical package names to package manager entries. By default the logical name, package name, and binary name are all the same thing. Overrides exist at the package level and per package manager.

```yaml
packages:
  fzf:
    # uses package manager defaults — no overrides needed
    brew: {}
    apt: {}
    dnf: {}
    pacman: {}

  ripgrep:
    binary: rg                # binary name differs from package name
    brew: {}
    apt: {}
    dnf: {}
    pacman: {}

  python-dateutil:
    # different package name per package manager
    pip:
      package: python-dateutil
    apt:
      package: python3-dateutil

  some-tool:
    # fully custom install command for one package manager
    brew:
      install: brew tap someorg/sometool && brew install some-tool
      check: which some-tool
    apt: {}

  no-binary-lib:
    check: "python3 -c 'import somelib'"   # no binary — custom check
    pip:
      package: somelib
```

---

## Module-level Package Overrides

A module can override the global registry for its own packages:

```yaml
packages:
  requires:
    - fzf
  overrides:
    fzf:
      brew:
        install: brew install fzf --HEAD
```

---

## Detection and Priority

The tool detects available package managers via `which`. Priority order is declared in `config.yaml` and overridable in `env.yaml`. The resolver walks the priority list and uses the first manager that has an entry for the requested package — so a package only in `cargo` uses `cargo` even if `brew` is preferred.

```yaml
# config.yaml
package_managers:
  priority: [brew, apt, dnf, pacman, pip, cargo]

# env.yaml override
package_managers:
  priority: [apt, pip]
```

---

## Install Check

Before installing, the tool runs `which {binary}` where `binary` defaults to the package name. Overridable via the `binary` key at the package level, or a fully custom `check` command for packages with no binary. Use `--force` to reinstall already-present packages.

---

## Core Design Principles

- Logical name = package name = binary name by default
- Overridable at any level (global, package, per-package-manager)
- Package manager commands are templates with `{package}` and `{binary}` placeholders
- Priority-ordered manager selection per package
- Standalone tool — separate from dot-dagger core
