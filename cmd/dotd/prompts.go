package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/rocne/dot-dagger/internal/ui"
)

// promptConfirm prints "Proceed? [y/N]: ", reads a line, and returns true only
// on "y" or "yes". Any other input (including empty / Enter) prints "cancelled"
// and returns false — callers should return nil when false.
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
