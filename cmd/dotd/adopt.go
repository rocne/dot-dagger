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

When the destination directory's .dagger rules would derive a different
symlink location than the file's original path (the default config/ scaffold
links into $config, but ~/.gitconfig lives in ~), adopt records the original
location as a files: entry in that .dagger, so a later 'dotd apply' re-creates
exactly the symlink adopt made — on this machine and any other.

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

	opts := adopter.AdoptOptions{
		DotfilesRoot: cfg.files,
		Conventions:  conv,
		HomeDir:      cfg.home,
		ConfigDir:    cfg.configDir,
		BinDir:       cfg.binDir,
		Force:        cfg.force,
	}

	// Dry-run stops here — nothing moves, so no confirmation is needed.
	if cfg.dryRun {
		destAbs := filepath.Join(cfg.files, destRel)
		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "dry-run: adopt %s → %s\n", srcAbs, destAbs)
		if persist := adopter.PlanPersist(srcAbs, destRel, opts); persist.Needed {
			fmt.Fprintf(out, "dry-run: record link destination %s in %s (%s)\n",
				persist.Dest, persist.DaggerPath, persistReason(persist))
		}
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

	res, persist, err := adopter.Adopt(srcAbs, destRel, opts)
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
		fmt.Fprintf(out, "%s  %s %s %s\n", ui.OK("adopted"), srcAbs, ui.Arrow("→"), destAbs)
		fmt.Fprintf(out, "added to shellrc/ — run %s to regenerate init.sh\n", ui.Key("dotd apply"))
	}

	switch {
	case persist == nil || !persist.Needed:
		// Nothing to record — apply already derives the same destination
		// (or the file is sourced, not linked).
	case persist.Persisted:
		fmt.Fprintf(out, "%s  link destination %s in %s (%s)\n",
			ui.OK("recorded"), persist.Dest, persist.DaggerPath, persistReason(persist))
	default:
		// Could not mutate .dagger safely — the adopt itself succeeded, so
		// instruct instead of failing (issue #191: without this entry the
		// next apply creates a second, wrong symlink).
		ui.Warnf(cmd.ErrOrStderr(), "could not record the link destination in %s automatically: %v", persist.DaggerPath, persist.Err)
		fmt.Fprintf(out, "add this to %s so %s re-creates the symlink at %s:\n\n%s\n",
			persist.DaggerPath, ui.Key("dotd apply"), persist.Dest, persist.Snippet)
	}
	return nil
}

// persistReason explains why adopt records (or would record) a link
// destination override for the adopted file.
func persistReason(p *adopter.PersistResult) string {
	if p.Derived == "" {
		return "no .dagger rule would otherwise link this file"
	}
	return fmt.Sprintf("apply would otherwise link it to %s", p.Derived)
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
