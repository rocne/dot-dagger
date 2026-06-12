# Load order

`init.sh` sources your shell scripts in a specific order. That order is determined by `@after` annotations, which declare dependencies between scripts.

## Why order matters

Shell scripts frequently depend on each other. Path setup must run before anything that uses the path. Base environment variables must be set before aliases that reference them. Plugin managers must load before their plugins.

Without explicit ordering, dotd falls back to alphabetical order by logical name. That's fine for independent scripts, but breaks down the moment one script needs something another script provides.

## Declaring dependencies with @after

Add `@after` to a file to declare that it should be sourced after another file or directory:

```sh
# shellrc/aliases.sh
# @after shellrc/base/
# @after shellrc/path/
```

```sh
# shellrc/fzf.sh
# @after shellrc/tools/
```

### Depending on a directory

A path ending in `/` means "after all active files under this path":

```sh
# @after shellrc/base/    # after all active files under shellrc/base/
# @after shellrc/env/     # after all active files under shellrc/env/
```

"Active" means the file passes its `@when` condition on this machine. If no files under the path are active, the dependency is silently ignored — it's never an error to `@after` something that doesn't exist on this machine.

### Depending on a specific file

A path without a trailing `/` is matched against [logical names](file-identity.md):

```sh
# @after shellrc.base           # the file whose logical name is shellrc.base
# @after shellrc.tmux.helpers   # a specific helper file
```

## How ordering works

dotd builds a dependency graph from all `@after` declarations across active files, then produces a topological ordering — an ordering where every dependency comes before the file that depends on it.

If there's a cycle (A depends on B, B depends on A), `dotd apply` fails with an error listing the cycle.

Files with no `@after` are ordered alphabetically by logical name within their position in the graph.

## Example

Given these files:

```
shellrc/
  base.sh            # no @after
  path.sh            # @after shellrc/base/
  aliases.sh         # @after shellrc/path/
  fzf.sh             # @after shellrc/aliases/
  homebrew.sh        # @after shellrc/base/, @when os=macos
```

On macOS, `init.sh` sources them in this order:

1. `base.sh`
2. `homebrew.sh` (depends on base; sibling of path.sh, alphabetically before)
3. `path.sh` (depends on base)
4. `aliases.sh` (depends on path)
5. `fzf.sh` (depends on aliases)

On Linux (where `homebrew.sh` is excluded by `@when os=macos`):

1. `base.sh`
2. `path.sh`
3. `aliases.sh`
4. `fzf.sh`

## Checking the load order

```sh
dotd dag check
```

This prints the load order for the current machine without writing `init.sh`.
