// Package ui provides color helpers for CLI output.
// Colors are automatically disabled when output is not a terminal or when
// NO_COLOR is set.
package ui

import (
	"fmt"
	"io"

	"github.com/fatih/color"
)

// Print helpers — write a labeled, colored line to w.
//
// Warnf/Errf/Hintf prefix the message with a colored label.
// OKf/Skipf color the entire message.
// Headerf emits a blank line, then a bold header line (section start).

func Warnf(w io.Writer, format string, a ...any) {
	fmt.Fprintf(w, "%s %s\n", Missing("warning:"), fmt.Sprintf(format, a...))
}

func Errf(w io.Writer, format string, a ...any) {
	fmt.Fprintf(w, "%s %s\n", Conflict("error:"), fmt.Sprintf(format, a...))
}

// Hintf renders the "hint:" line that follows an error (see the Hinter
// convention in cmd/dotd/errors.go) — the single owner of hint styling.
func Hintf(w io.Writer, format string, a ...any) {
	fmt.Fprintf(w, "%s %s\n", Arrow("hint:"), fmt.Sprintf(format, a...))
}

func OKf(w io.Writer, format string, a ...any) {
	fmt.Fprintf(w, "%s\n", OK(fmt.Sprintf(format, a...)))
}

func Skipf(w io.Writer, format string, a ...any) {
	fmt.Fprintf(w, "%s\n", Skip(fmt.Sprintf(format, a...)))
}

// Headerf writes a blank line followed by a bold section header.
// All standalone section headers in the CLI use this leading-newline pattern.
func Headerf(w io.Writer, format string, a ...any) {
	fmt.Fprintf(w, "\n%s\n", Header(fmt.Sprintf(format, a...)))
}

func Missingf(w io.Writer, format string, a ...any) {
	fmt.Fprintf(w, "%s %s\n", Missing("missing:"), fmt.Sprintf(format, a...))
}

func Wrongf(w io.Writer, format string, a ...any) {
	fmt.Fprintf(w, "%s %s\n", Wrong("stale:"), fmt.Sprintf(format, a...))
}

// Semantic Sprint functions — each returns a colored string.

func OK(s string) string          { return green.Sprint(s) }
func Missing(s string) string     { return yellow.Sprint(s) }
func Wrong(s string) string       { return red.Sprint(s) }
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
