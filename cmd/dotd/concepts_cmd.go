package main

import (
	"fmt"
	"strings"

	"github.com/rocne/dot-dagger/internal/predicate"
	"github.com/spf13/cobra"
)

var conceptsText = strings.ReplaceAll(`dotd — concepts reference
═════════════════════════

PIPELINE
  dotd processes dotfiles in five stages:

  walk      Traverse the dotfiles repo, read .dagger annotation files and
            file headers to build a list of nodes.
  filter    Evaluate each node's @when predicate against the resolved env
            map. Nodes whose condition is false are excluded.
  order     Topological sort: nodes listed in @after dependencies come first.
  act       Execute actions: create symlinks, record sourced files, assemble
            compose targets.
  init.sh   Regenerate the shell init file from all sourced nodes.

PREDICATES (@when)
  @when controls whether a node is active. Syntax:

__PREDICATE_SYNTAX__

  Comma separates multiple values for ONE key.
  Use AND/OR to join two separate conditions.

ANNOTATIONS
  Written as comments in file headers (e.g. # @when(os=macos)).
  Managed interactively with: dotd annotate <file>

    @when(expr)        Condition for activation (see PREDICATES above)
    @action(type)      How dotd processes this file: source, no-source, link
    @after(name)       Logical name this file must load after
    @name(name)        Override the logical name used in the dependency graph
    @require(pkg)      Package that must be installed (blocks activation)
    @request(pkg)      Package to install if missing (non-blocking)
    @disable           Exclude this file from all processing

ENV.YAML
  A flat YAML file mapping string keys to string values:

    os: $(dotd get-os)
    hostname: $(hostname)
    context: work

  Shell expressions: values matching $(…) are evaluated via sh -c each
  time dotd runs. Use single quotes to store them without shell expansion:

    dotd env set os '$(dotd get-os)'

  Resolution order (highest priority wins):
    1. --env flags        e.g. --env context=work
    2. DOTD_* shell vars  e.g. DOTD_CONTEXT=work
    3. env.yaml values

  Commands: dotd env show | get | set | edit | diff | path

DIRECTORY NAMING
  dotd interprets directory and file name prefixes:

    dot-bashrc      links/names as .bashrc  (leading dot added)
    nosync-work/    strips "nosync-"        (avoids sync-tool ignore rules)
    shellrc.d/      compose target          (contents assembled into shellrc)
`, "__PREDICATE_SYNTAX__", predicate.IndentSyntaxHelp("    ")+"\n")

func newConceptsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "concepts",
		Short: "Print a reference of dotd concepts and syntax",
		Long: `Print a single-page reference of dotd concepts: pipeline stages,
@when predicates, annotations, env.yaml, and directory naming conventions.

Examples:
  dotd concepts
  dotd concepts | less
  dotd concepts | grep -A4 PREDICATES`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprint(cmd.OutOrStdout(), conceptsText)
			return nil
		},
	}
}
