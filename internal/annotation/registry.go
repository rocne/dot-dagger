package annotation

import (
	"fmt"
	"io"
	"os"
)

// Mode controls how unknown annotation keys are handled during dispatch.
type Mode int

const (
	// Strict causes an unknown annotation key to return an error. This is the default.
	Strict Mode = iota
	// Warn causes an unknown annotation key to log a warning and continue.
	Warn
)

// Handler is called when a registered annotation key is encountered.
// dryRun is true when no side effects should be produced; handlers must
// respect this and skip any writes or external calls.
type Handler func(value string, dryRun bool) error

// Registry holds registered annotation handlers and controls behaviour for
// unknown keys. The zero value is not usable; use NewRegistry.
type Registry struct {
	mode     Mode
	handlers map[string]Handler
	warnOut  io.Writer
}

// NewRegistry returns a Registry with the given mode.
// Warning messages are written to os.Stderr by default.
func NewRegistry(mode Mode) *Registry {
	return &Registry{
		mode:     mode,
		handlers: make(map[string]Handler),
		warnOut:  os.Stderr,
	}
}

// SetWarnOutput sets the writer used for Warn mode messages.
// Useful for capturing warnings in tests.
func (r *Registry) SetWarnOutput(w io.Writer) {
	r.warnOut = w
}

// Register registers h as the handler for key.
// Panics if key is already registered.
func (r *Registry) Register(key string, h Handler) {
	if _, exists := r.handlers[key]; exists {
		panic(fmt.Sprintf("annotation: key %q already registered", key))
	}
	r.handlers[key] = h
}

// Dispatch calls the registered handler for ann.Key.
// Core annotation keys (see IsCoreKey) are silently skipped.
// If no handler is registered for ann.Key, behaviour depends on Mode:
//   - Strict: returns an error
//   - Warn: writes a warning to the warn output and returns nil
func (r *Registry) Dispatch(ann Annotation, dryRun bool) error {
	if IsCoreKey(ann.Key) {
		return nil
	}
	h, ok := r.handlers[ann.Key]
	if !ok {
		switch r.mode {
		case Strict:
			return fmt.Errorf("annotation: unknown key %q at line %d", ann.Key, ann.Line)
		case Warn:
			fmt.Fprintf(r.warnOut, "warning: unknown annotation key %q at line %d\n", ann.Key, ann.Line)
			return nil
		}
	}
	return h(ann.Value, dryRun)
}

// DispatchAll calls Dispatch for each annotation in anns.
// Returns the first error encountered, or nil if all succeed.
func (r *Registry) DispatchAll(anns []Annotation, dryRun bool) error {
	for _, a := range anns {
		if err := r.Dispatch(a, dryRun); err != nil {
			return err
		}
	}
	return nil
}
