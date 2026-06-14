// Package log configures the shared logger for dotd.
// Status and diagnostic output goes to stderr via this logger.
// Data output (pipeable results) goes directly to stdout via cobra commands.
//
// Levels: debug < info < warn < error
//
//	debug — per-item detail and internals (path resolution, predicate eval)
//	info  — stage summaries; default level
//	warn  — recoverable issues
//	error — failures
//
// Info messages have no level prefix so normal output looks unchanged.
// Debug/warn/error messages are prefixed (DEBU/WARN/ERRO).
package log

import (
	"io"

	"github.com/charmbracelet/lipgloss"
	chlog "github.com/charmbracelet/log"
)

// New returns a logger writing to w at the given level name.
// Pass os.Stderr for production use; pass a test buffer for tests.
// Valid level names: "debug", "info", "warn", "error".
// Returns an error if the level name is unrecognised.
func New(w io.Writer, level string) (*chlog.Logger, error) {
	lvl, err := chlog.ParseLevel(level)
	if err != nil {
		return nil, err
	}

	styles := chlog.DefaultStyles()
	// Info messages carry no level prefix — the ui.Header prefix in the
	// message itself already serves that role, so "INFO" would be redundant.
	styles.Levels[chlog.InfoLevel] = lipgloss.NewStyle()

	logger := chlog.NewWithOptions(w, chlog.Options{
		Level:           lvl,
		ReportTimestamp: false,
	})
	logger.SetStyles(styles)

	return logger, nil
}

// LevelNames returns the accepted level name strings for use in flag help text.
func LevelNames() string {
	return "debug|info|warn|error"
}
