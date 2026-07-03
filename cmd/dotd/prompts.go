package main

// Prompt conventions:
//   - All prompts default to a SAFE choice when stdin is EOF (no, cancel, skip).
//   - When the user is interactive, [Y/n] vs [y/N] indicates the Enter-default.
//   - [Y/n]: Enter → yes.  [y/N]: Enter → no.
//   - Never auto-accept a destructive or filesystem-mutating action on EOF.
//
// huh helpers (promptMenu, promptSelect, promptInput, promptBool, promptInputs):
//   - All route through runForm → cmd.InOrStdin() / cmd.ErrOrStderr().
//   - Non-TTY contexts (tests, CI) automatically use huh accessible mode:
//     numbered menus, line-buffered text. Driveable by cmd.SetIn(strings.NewReader(...)).
//   - An exhausted non-TTY stdin aborts (errUserAborted) instead of silently
//     accepting field defaults — see runForm.
//   - This file is the only file that imports charmbracelet/huh.

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/x/term"
	"github.com/rocne/dot-dagger/internal/ui"
	"github.com/spf13/cobra"
)

// errUserAborted is returned when the user cancels an interactive prompt.
// main() maps this sentinel to a clean exit-1 with "cancelled" on stderr,
// avoiding the noisy "Error: user aborted" that Cobra would otherwise print.
var errUserAborted = errors.New("user aborted")

// isTTY reports whether r is an interactive terminal.
// Returns false for any reader without an fd — e.g. strings.Reader in tests,
// piped stdin in CI. Use isTTY(cmd.InOrStdin()) instead of os.Stdin.Fd() directly.
func isTTY(r io.Reader) bool {
	if f, ok := r.(interface{ Fd() uintptr }); ok {
		return term.IsTerminal(f.Fd())
	}
	return false
}

// byteReader limits reads to one byte at a time to prevent bufio.Scanner
// from over-reading across sequential huh form boundaries in accessible mode.
// huh creates a new bufio.Scanner per field; in non-TTY contexts all pipe
// bytes are available at once, so without this the first scanner consumes
// all subsequent fields' input.
//
// It also records delivery stats so runForm can distinguish a real piped
// answer from an exhausted reader (see runForm).
type byteReader struct {
	r   io.Reader
	n   int  // bytes delivered
	eof bool // underlying reader reported io.EOF
}

func (b *byteReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	n, err := b.r.Read(p[:1])
	b.n += n
	if errors.Is(err, io.EOF) {
		b.eof = true
	}
	return n, err
}

// runForm builds a huh form wired to cmd's stdin and stderr, runs it, and
// normalizes the outcome. When stdin is not a terminal (tests, CI, piped
// input) it automatically enables accessible mode: plain numbered menus and
// line-buffered text input.
//
// huh.ErrUserAborted maps to errUserAborted. An exhausted non-TTY reader is
// also an abort: on EOF, huh's accessible mode fills every field with its
// default and returns nil — fabricating an answer the user never gave. For a
// prompt presented in a loop (the annotate menu) that meant re-selecting the
// first option forever. A run that consumed at least one byte before hitting
// EOF is a real answer (piped input without a trailing newline), not an abort.
func runForm(cmd *cobra.Command, fields ...huh.Field) error {
	r := cmd.InOrStdin()
	tty := isTTY(r)
	input := io.Reader(r)
	var br *byteReader
	if !tty {
		br = &byteReader{r: r}
		input = br
	}
	err := huh.NewForm(huh.NewGroup(fields...)).
		WithAccessible(!tty).
		WithInput(input).
		WithOutput(cmd.ErrOrStderr()).
		Run()
	if errors.Is(err, huh.ErrUserAborted) {
		return errUserAborted
	}
	if err != nil {
		return err
	}
	if br != nil && br.eof && br.n == 0 {
		return errUserAborted
	}
	return nil
}

// promptMenu presents a numbered selection menu and returns the zero-based index
// of the chosen option. Callers typically make the last option "Done".
func promptMenu(cmd *cobra.Command, title string, options []string) (int, error) {
	var chosen string
	err := runForm(cmd,
		huh.NewSelect[string]().
			Title(title).
			Options(huh.NewOptions(options...)...).
			Value(&chosen),
	)
	if err != nil {
		return 0, err
	}
	for i, o := range options {
		if o == chosen {
			return i, nil
		}
	}
	return 0, fmt.Errorf("promptMenu: unknown selection %q", chosen)
}

// promptSelect presents a labeled selection with description and returns the chosen value.
func promptSelect(cmd *cobra.Command, title, desc string, options []string) (string, error) {
	var chosen string
	err := runForm(cmd,
		huh.NewSelect[string]().
			Title(title).
			Description(desc).
			Options(huh.NewOptions(options...)...).
			Value(&chosen),
	)
	return strings.TrimSpace(chosen), err
}

// promptInput shows a text field pre-filled with prefill and returns the trimmed value.
// An empty result (user clears the field) is allowed; callers treat it as "remove".
// validate is called on non-empty input; pass nil to skip validation.
func promptInput(cmd *cobra.Command, title, desc, prefill string, validate func(string) error) (string, error) {
	val := prefill
	err := runForm(cmd,
		huh.NewInput().
			Title(title).
			Description(desc).
			Value(&val).
			Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return nil // empty = remove; always valid
				}
				if validate != nil {
					return validate(s)
				}
				return nil
			}),
	)
	return strings.TrimSpace(val), err
}

// promptBool shows a yes/no confirm pre-set to initial and returns the chosen value.
func promptBool(cmd *cobra.Command, title, desc, affirm, neg string, initial bool) (bool, error) {
	val := initial
	err := runForm(cmd,
		huh.NewConfirm().
			Title(title).
			Description(desc).
			Affirmative(affirm).
			Negative(neg).
			Value(&val),
	)
	return val, err
}

// inputPrompt describes a single text input field for use with promptInputs.
type inputPrompt struct {
	Key      string
	Title    string
	Validate func(string) error
}

// promptInputs presents a form with one text field per prompt entry and returns
// a map of Key → trimmed value. All fields are required; empty input is rejected.
func promptInputs(cmd *cobra.Command, prompts []inputPrompt) (map[string]string, error) {
	vals := make([]string, len(prompts))
	fields := make([]huh.Field, len(prompts))
	for i, p := range prompts {
		i, p := i, p
		validate := p.Validate
		fields[i] = huh.NewInput().
			Title(p.Title).
			Value(&vals[i]).
			Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return fmt.Errorf("value required")
				}
				if validate != nil {
					return validate(s)
				}
				return nil
			})
	}
	if err := runForm(cmd, fields...); err != nil {
		return nil, err
	}
	result := make(map[string]string, len(prompts))
	for i, p := range prompts {
		result[p.Key] = strings.TrimSpace(vals[i])
	}
	return result, nil
}

// promptConfirm prints "Proceed? [y/N]: ", reads a line, and returns true only
// on "y" or "yes". Any other input (including empty / Enter or EOF) prints
// "cancelled" and returns false — callers should return nil when false.
func promptConfirm(out io.Writer, r io.Reader) bool {
	fmt.Fprint(out, "\nProceed? [y/N]: ")
	ans, _ := bufio.NewReader(r).ReadString('\n')
	ans = strings.TrimSpace(strings.ToLower(ans))
	if ans != "y" && ans != "yes" {
		ui.Skipf(out, "cancelled")
		return false
	}
	return true
}

// promptDefault prints "msg [default]: " and reads input.
// Returns defaultVal if input is empty.
func promptDefault(w io.Writer, reader *bufio.Reader, msg, defaultVal string, nonInteractive bool) (string, error) {
	if nonInteractive {
		return defaultVal, nil
	}
	if defaultVal != "" {
		fmt.Fprintf(w, "%s [%s]: ", msg, ui.Skip(defaultVal))
	} else {
		fmt.Fprintf(w, "%s: ", msg)
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		fmt.Fprintln(w)        // terminate the dangling prompt line
		return defaultVal, nil // EOF — use default
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal, nil
	}
	return line, nil
}

// promptYN prints "msg [Y/n]: " and returns true unless user types n/no.
// Interactive: empty input (Enter) defaults to yes.
// Non-interactive: EOF returns false (safe default — never auto-accept on closed stdin).
func promptYN(w io.Writer, reader *bufio.Reader, msg string) (bool, error) {
	fmt.Fprintf(w, "  %s [Y/n]: ", msg)
	line, err := reader.ReadString('\n')
	if err != nil {
		fmt.Fprintln(w)   // terminate the dangling prompt line
		return false, nil // EOF → safe default: no
	}
	line = strings.ToLower(strings.TrimSpace(line))
	return line == "" || line == "y" || line == "yes", nil
}

// printField prints a bold field label and a faint description, then a blank line.
func printField(w io.Writer, label, desc string) {
	fmt.Fprintf(w, "\n  %s\n", ui.Key(label))
	fmt.Fprintf(w, "  %s\n", ui.Skip(desc))
}

// fieldPrompt returns the prompt text used after a printField call.
func fieldPrompt() string {
	return "  " + ui.Arrow("›")
}
