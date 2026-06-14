package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rocne/dot-dagger/internal/adopter"
	"github.com/rocne/dot-dagger/internal/dagger"
	"github.com/rocne/dot-dagger/internal/ecosystem"
	"github.com/rocne/dot-dagger/internal/ui"
	"github.com/spf13/cobra"
)

func newAdoptCmd(rootCfg *config) *cobra.Command {
	var (
		to  string
		yes bool
	)

	cmd := &cobra.Command{
		Use:   "adopt <file>",
		Short: "Move a file into the dotfiles repo and replace it with a symlink",
		Long: `Move a file into the dotfiles repo and replace it with a symlink.

Destination is inferred from the file type:

  Executable bit set           →  bin/<name>
  .sh / .bash / .zsh / .fish   →  shellrc/<name>
  Hidden file (.bashrc, …)     →  config/dot-<name>   (dot- prefix added)
  .conf / .toml / .yaml / …    →  config/<name>

Use --to to override the inferred destination. If inference fails and --to
is not provided, the command errors.

For shellrc/ files, no symlink is created. The file is sourced via init.sh
after running dotd apply.

Examples:
  dotd adopt ~/.bashrc                              # → config/dot-bashrc
  dotd adopt ~/bin/my-script                        # → bin/my-script
  dotd adopt ~/.gitconfig --to config/dot-gitconfig-work
  dotd adopt ~/.zshrc --yes                         # skip confirmation`,
		Args: usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAdopt(cmd, rootCfg, args[0], to, yes)
		},
	}
	cmd.Flags().StringVar(&to, "to", "", "destination path relative to dotfiles root (overrides inference)")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation prompt")
	return cmd
}

func runAdopt(cmd *cobra.Command, cfg *config, src, to string, yes bool) error {
	// Resolve src to absolute path.
	srcAbs, err := filepath.Abs(src)
	if err != nil {
		return fmt.Errorf("adopt: resolve path: %w", err)
	}

	info, err := os.Stat(srcAbs)
	if err != nil {
		return fmt.Errorf("adopt: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("adopt: %s is a directory — adopt one file at a time", srcAbs)
	}

	dagCfg, err := dagger.LoadFile(filepath.Join(cfg.files, ecosystem.ConfigFile))
	if err != nil {
		return fmt.Errorf("adopt: load %s: %w", ecosystem.ConfigFile, err)
	}
	conv := conventionsFrom(dagCfg)

	// Resolve destination.
	var destRel string
	if to != "" {
		destRel = resolveToFlag(to, info.Name())
	} else {
		inf := adopter.Infer(srcAbs, info, conv)
		if inf.Unknown {
			return fmt.Errorf("adopt: cannot infer destination for %q — use --to <path>", info.Name())
		}
		destRel = inf.DestRel
		cfg.log.Debugf("inferred destination: %s (%s)", destRel, inf.Reason)
	}

	// Dry-run stops here — nothing moves, so no confirmation is needed.
	if cfg.dryRun {
		destAbs := filepath.Join(cfg.files, destRel)
		fmt.Fprintf(cmd.OutOrStdout(), "dry-run: adopt %s → %s\n", srcAbs, destAbs)
		return nil
	}

	// Confirmation prompt. --yes skips it. A non-TTY stdin without --yes is
	// refused — adopt moves a file, and prompts must never auto-accept a
	// mutating action on piped or closed stdin (see prompts.go conventions).
	if !yes {
		if !isTTY(cmd.InOrStdin()) {
			return &hintError{
				err:  errors.New("adopt: confirmation required on non-interactive stdin"),
				hint: "pass --yes (-y) to adopt without prompting",
			}
		}
		confirmed, err := promptBool(cmd,
			fmt.Sprintf("Adopt %s → %s and replace with symlink?", srcAbs, destRel),
			"", "Yes", "No, cancel", false)
		if err != nil {
			return fmt.Errorf("adopt: %w", err)
		}
		if !confirmed {
			ui.Skipf(cmd.OutOrStdout(), "cancelled")
			return nil
		}
	}

	opts := adopter.AdoptOptions{
		DotfilesRoot: cfg.files,
		Conventions:  conv,
		HomeDir:      cfg.home,
		ConfigDir:    cfg.configDir,
		BinDir:       cfg.binDir,
		Force:        cfg.force,
	}

	res, err := adopter.Adopt(srcAbs, destRel, opts)
	if err != nil {
		return err
	}

	destAbs := filepath.Join(cfg.files, destRel)

	// Mutation result → stdout, never suppressed by --quiet (channel policy,
	// 2026-06-13 audit O1).
	out := cmd.OutOrStdout()
	if len(res.Links) > 0 {
		fmt.Fprintf(out, "%s  %s %s %s\n",
			ui.OK("adopted"), res.Links[0].Dest, ui.Arrow("→"), destAbs)
	} else {
		fmt.Fprintf(out, "%s  %s → %s\n", ui.OK("adopted"), srcAbs, destAbs)
		fmt.Fprintf(out, "added to shellrc/ — run %s to regenerate init.sh\n", ui.Header("dotd apply"))
	}
	return nil
}

// resolveToFlag resolves the --to flag to a relative destination path.
// If to ends with a path separator, src name is appended.
func resolveToFlag(to, name string) string {
	if strings.HasSuffix(to, "/") || strings.HasSuffix(to, string(os.PathSeparator)) {
		return filepath.Join(to, name)
	}
	return to
}

// conventionsFrom builds a ConventionNames from a loaded .dagger config,
// applying defaults for any empty fields.
func conventionsFrom(cfg *dagger.ComposableNode) adopter.ConventionNames {
	conv := adopter.DefaultConventions()
	if cfg.Conventions.Shellrc != "" {
		conv.Shellrc = cfg.Conventions.Shellrc
	}
	if cfg.Conventions.Bin != "" {
		conv.Bin = cfg.Conventions.Bin
	}
	if cfg.Conventions.Config != "" {
		conv.Config = cfg.Conventions.Config
	}
	return conv
}

