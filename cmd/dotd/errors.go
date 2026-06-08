package main

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
