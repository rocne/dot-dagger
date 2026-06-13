package pipeline

import (
	"fmt"
	"strings"

	"github.com/rocne/dot-dagger/internal/ecosystem"
	"github.com/rocne/dot-dagger/internal/fileutil"
)

// GenerateOptions configures init.sh generation.
type GenerateOptions struct {
	BinDir string // if non-empty, prepended to PATH in init.sh
}

// Generate writes init.sh to path from the ordered list of nodes to source.
// Uses POSIX ". path" syntax with shell-quoted paths.
// Writes atomically via fileutil.WriteAtomic.
func Generate(path string, nodes []RawNode, opts GenerateOptions) error {
	content := buildInitSh(nodes, opts)
	if err := fileutil.WriteAtomic(path, []byte(content), fileutil.ModeFile); err != nil {
		return fmt.Errorf("initgen: %w", err)
	}
	return nil
}

func buildInitSh(nodes []RawNode, opts GenerateOptions) string {
	var b strings.Builder
	b.WriteString(ecosystem.GeneratedFileHeader() + "\n\n")
	if opts.BinDir != "" {
		fmt.Fprintf(&b, "export PATH=\"%s:${PATH}\"\n\n", opts.BinDir)
	}
	for _, n := range nodes {
		fmt.Fprintf(&b, ". %s\n", fileutil.ShellQuote(n.Path))
	}
	return b.String()
}
