// Package ui provides color helpers for CLI output.
// Colors are automatically disabled when output is not a terminal or when
// NO_COLOR is set.
package ui

import "github.com/fatih/color"

// Semantic Sprint functions — each returns a colored string.

func OK(s string) string          { return green.Sprint(s) }
func Missing(s string) string     { return yellow.Sprint(s) }
func Wrong(s string) string       { return yellow.Sprint(s) }
func Conflict(s string) string    { return red.Sprint(s) }
func Installed(s string) string   { return green.Sprint(s) }
func Installable(s string) string { return cyan.Sprint(s) }
func Skip(s string) string        { return faint.Sprint(s) }
func Install(s string) string     { return cyan.Sprint(s) }
func HardMissing(s string) string { return redBold.Sprint(s) }
func Header(s string) string      { return bold.Sprint(s) }
func Arrow(s string) string       { return cyan.Sprint(s) }
func Key(s string) string         { return boldCyan.Sprint(s) }
func Count(n int, fn func(string) string, zero func(string) string) func(string) string {
	if n > 0 {
		return fn
	}
	return zero
}

var (
	green    = color.New(color.FgGreen)
	yellow   = color.New(color.FgYellow)
	red      = color.New(color.FgRed)
	cyan     = color.New(color.FgCyan)
	bold     = color.New(color.Bold)
	boldCyan = color.New(color.FgCyan, color.Bold)
	redBold  = color.New(color.FgRed, color.Bold)
	faint    = color.New(color.Faint)
)
