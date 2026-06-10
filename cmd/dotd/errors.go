package main

import (
	"errors"

	"github.com/spf13/cobra"
)

// Error rendering convention:
//   error: <message>
//   hint:  <recommended action>
//
// Any error returned from a RunE that implements Hinter will have its hint
// rendered on its own line by the root error handler in main.go. Use hintError
// to attach a hint without polluting the Error() string with hint text.

// Hinter is implemented by errors that want to surface a separate hint line
// after the rendered error. The root handler discovers it via errors.As.
type Hinter interface {
	Hint() string
}

// hintError wraps an error with an associated hint message. Error() returns
// only the underlying message — the hint is rendered separately by the root
// handler so the wrapped error chain stays clean.
type hintError struct {
	err  error
	hint string
}

func (e *hintError) Error() string { return e.err.Error() }
func (e *hintError) Unwrap() error { return e.err }
func (e *hintError) Hint() string  { return e.hint }

// usageError marks an error as caused by invalid CLI usage (bad flag, wrong
// arg count, unknown subcommand key, etc.) rather than a runtime failure.
// The root handler exits 2 for usage errors and 1 for everything else, so
// scripts can distinguish "you invoked me wrong" from "the operation failed".
// usageError unwraps to its inner error, so hint chaining via hintError keeps
// working transparently (a hintError wrapped in usageError still surfaces the
// hint line).
type usageError struct {
	err error
}

func (e *usageError) Error() string { return e.err.Error() }
func (e *usageError) Unwrap() error { return e.err }

// asUsageError wraps err as a usageError unless err is already one. nil in,
// nil out. Useful inside cobra arg validators and FlagErrorFunc.
func asUsageError(err error) error {
	if err == nil {
		return nil
	}
	var ue *usageError
	if errors.As(err, &ue) {
		return err
	}
	return &usageError{err: err}
}

// usageArgs wraps a cobra.PositionalArgs so validation failures surface as
// usageError. Use it on commands whose arg shape is checked via cobra
// helpers like cobra.ExactArgs.
func usageArgs(inner cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		return asUsageError(inner(cmd, args))
	}
}
