package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/x/term"
	"github.com/rocne/dot-dagger/internal/ui"
	"github.com/rocne/dot-dagger/internal/walk"
	"github.com/spf13/cobra"
)

func newAdoptCmd(rootCfg *config) *cobra.Command {
	var (
		to            string
		yes           bool
		noInteractive bool
	)

	cmd := &cobra.Command{
		Use:   "adopt <file>",
		Short: "Import a file into the dotfiles repo, inferring the destination directory",
		Long: `Copy a file into the dotfiles repo. The destination is inferred from the file:

  Executable bit set          →  bin/<name>
  .sh / .bash / .zsh / .fish  →  shellrc/<name>
  Hidden file (.bashrc, …)    →  conf/dot-<name>   (dot- prefix added)
  .conf / .toml / .yaml / …   →  conf/<name>

Use --to to override the inferred destination. If inference fails and --to is
not provided, the command errors.

Examples:
  dotd adopt ~/.bashrc                         # → conf/dot-bashrc
  dotd adopt ~/bin/my-script                   # → bin/my-script
  dotd adopt ~/.gitconfig --to conf/dot-gitconfig-work
  dotd adopt ~/.zshrc --yes                    # skip confirmation prompt`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			nonInteractive := yes || noInteractive
			return runAdopt(cmd, rootCfg, args[0], to, nonInteractive)
		},
	}
	cmd.Flags().StringVar(&to, "to", "", "destination path relative to dotfiles root (overrides inference)")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "non-interactive: accept inferred destination")
	cmd.Flags().BoolVar(&noInteractive, "no-interactive", false, "non-interactive: accept inferred destination")
	return cmd
}

func runAdopt(cmd *cobra.Command, cfg *config, src, to string, nonInteractive bool) error {
	if !nonInteractive && !term.IsTerminal(os.Stdin.Fd()) {
		nonInteractive = true
	}

	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("import: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("import: %s is a directory; import one file at a time", src)
	}

	// Resolve destination relative path.
	var destRel string
	if to != "" {
		destRel = resolveToFlag(to, info.Name())
	} else {
		inferred := inferImportDest(src, info)
		if inferred.unknown {
			if nonInteractive {
				return fmt.Errorf("import: cannot infer destination for %q; use --to", filepath.Base(src))
			}
			// Fall through to interactive prompt with empty pre-fill.
			destRel = ""
		} else {
			destRel = inferred.rel
		}

		if !nonInteractive {
			destRel, err = promptImportDest(src, destRel, inferred)
			if err != nil {
				return err
			}
			if destRel == "" {
				return fmt.Errorf("import: no destination specified")
			}
		}
	}

	destAbs := filepath.Join(cfg.files, destRel)

	// Don't overwrite silently.
	if _, err := os.Stat(destAbs); err == nil {
		return fmt.Errorf("import: destination already exists: %s", destAbs)
	}

	if err := copyFile(src, destAbs); err != nil {
		return err
	}
	cfg.log.Infof("%s  %s %s %s", ui.OK("imported"), src, ui.Arrow("→"), destAbs)

	// Offer to remove original (interactive only).
	if !nonInteractive {
		if err := offerRemoveOriginal(cfg, src); err != nil {
			return err
		}
	}

	return nil
}

// inferImportDest infers the destination directory and filename for src.
type importInference struct {
	rel     string // relative path from dotfiles root (e.g. "scripts/foo.sh")
	reason  string // human-readable description
	unknown bool   // true if we couldn't infer
}

func inferImportDest(src string, info os.FileInfo) importInference {
	name := info.Name()
	ext := strings.ToLower(filepath.Ext(name))

	// Executable bit → bin/
	if info.Mode()&0111 != 0 {
		return importInference{rel: filepath.Join(walk.DirBin, name), reason: "executable"}
	}

	// Shell script extensions → shellrc/
	switch ext {
	case ".sh", ".bash", ".zsh", ".fish":
		return importInference{rel: filepath.Join(walk.DirShellrc, name), reason: "shell script"}
	}

	// Hidden dotfile (e.g. .bashrc) → conf/dot-bashrc
	if strings.HasPrefix(name, ".") && len(name) > 1 {
		dotName := "dot-" + name[1:]
		return importInference{
			rel:    filepath.Join(walk.DirConf, dotName),
			reason: "dotfile (dot- prefix added for symlink naming)",
		}
	}

	// Config-like extensions → conf/
	switch ext {
	case ".conf", ".config", ".toml", ".yaml", ".yml", ".ini", ".cfg", ".json":
		return importInference{rel: filepath.Join(walk.DirConf, name), reason: "config file"}
	}

	return importInference{unknown: true}
}

// resolveToFlag resolves the --to flag value to a relative path.
// If to ends with a separator, it's treated as a directory and name is appended.
func resolveToFlag(to, name string) string {
	if strings.HasSuffix(to, "/") || strings.HasSuffix(to, string(os.PathSeparator)) {
		return filepath.Join(to, name)
	}
	return to
}

func promptImportDest(src, prefilledRel string, inferred importInference) (string, error) {
	var destRel string

	title := fmt.Sprintf("Destination for %s", filepath.Base(src))
	desc := ""
	if !inferred.unknown {
		desc = fmt.Sprintf("Inferred: %s", inferred.reason)
	} else {
		desc = "Could not infer — enter a path relative to dotfiles root"
	}

	destRel = prefilledRel
	if err := huh.NewInput().
		Title(title).
		Description(desc).
		Value(&destRel).
		Run(); err != nil {
		return "", fmt.Errorf("import: %w", err)
	}
	return strings.TrimSpace(destRel), nil
}

func offerRemoveOriginal(cfg *config, src string) error {
	var doRemove bool
	if err := huh.NewConfirm().
		Title(fmt.Sprintf("Remove original file %s?", src)).
		Affirmative("Yes, remove it").
		Negative("No, keep it").
		Value(&doRemove).
		Run(); err != nil {
		return fmt.Errorf("import: %w", err)
	}
	if doRemove {
		if err := os.Remove(src); err != nil {
			return fmt.Errorf("import: remove original: %w", err)
		}
		cfg.log.Infof("%s  %s", ui.Missing("removed"), src)
	}
	return nil
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("import: mkdir %s: %w", filepath.Dir(dst), err)
	}

	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("import: open %s: %w", src, err)
	}
	defer func() { _ = in.Close() }()

	srcInfo, err := in.Stat()
	if err != nil {
		return fmt.Errorf("import: stat %s: %w", src, err)
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_EXCL, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("import: create %s: %w", dst, err)
	}
	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, in); err != nil {
		_ = os.Remove(dst)
		return fmt.Errorf("import: copy to %s: %w", dst, err)
	}
	return nil
}
