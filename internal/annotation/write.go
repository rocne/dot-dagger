package annotation

import (
	"fmt"
	"os"
	"strings"

	"github.com/rocne/dot-dagger/internal/fileutil"
)

// Write atomically rewrites path with the given annotation block.
// preserved contains annotation lines for unknown keys (passed through unchanged).
// lines contains formatted annotation lines from the wizard's staged list.
//
// Stripping and insertion are bounded to the header block — the shebang plus the
// contiguous run of comment/blank lines up to the first code line, i.e. the same
// region annotation.Scan reads. Annotation-looking comments in the file body
// (e.g. "# @param" inside a function) are never touched.
//
// If both preserved and lines are empty, the header's annotations are stripped
// and no block is written.
func Write(path string, preserved []string, lines []string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("annotation: write: read %s: %w", path, err)
	}

	stat, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("annotation: write: stat %s: %w", path, err)
	}

	raw := strings.Split(string(data), "\n")
	// strings.Split on "a\nb\n" gives ["a","b",""] — drop trailing empty.
	if len(raw) > 0 && raw[len(raw)-1] == "" {
		raw = raw[:len(raw)-1]
	}

	// Header block = optional shebang + contiguous comment/blank lines, up to the
	// first code line. Matches annotation.Scan's scope so body comments survive.
	headerEnd := 0
	if len(raw) > 0 && strings.HasPrefix(strings.TrimSpace(raw[0]), "#!") {
		headerEnd = 1
	}
	for headerEnd < len(raw) {
		t := strings.TrimSpace(raw[headerEnd])
		if t == "" || strings.HasPrefix(t, "#") || strings.HasPrefix(t, "//") {
			headerEnd++
		} else {
			break
		}
	}
	body := raw[headerEnd:]

	// Strip existing annotation lines from the header only.
	strippedHeader := make([]string, 0, headerEnd)
	for _, l := range raw[:headerEnd] {
		if !IsAnnotationLine(l) {
			strippedHeader = append(strippedHeader, l)
		}
	}

	// Insertion point: after shebang + contiguous leading comment lines
	// (stops at the first blank or code line within the stripped header).
	insertAt := 0
	if len(strippedHeader) > 0 && strings.HasPrefix(strings.TrimSpace(strippedHeader[0]), "#!") {
		insertAt = 1
	}
	for insertAt < len(strippedHeader) {
		trimmed := strings.TrimSpace(strippedHeader[insertAt])
		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//") {
			insertAt++
		} else {
			break
		}
	}

	before := strippedHeader[:insertAt]
	afterHeader := strippedHeader[insertAt:]

	// Fresh slice — never append into the caller's preserved backing array.
	allAnnotations := append(append([]string{}, preserved...), lines...)

	var result []string
	result = append(result, before...)
	if len(allAnnotations) > 0 {
		if len(before) > 0 && strings.TrimSpace(before[len(before)-1]) != "" {
			result = append(result, "")
		}
		result = append(result, allAnnotations...)

		// Blank after the block, unless the next line is already blank.
		followingNonBlank := (len(afterHeader) > 0 && strings.TrimSpace(afterHeader[0]) != "") ||
			(len(afterHeader) == 0 && len(body) > 0 && strings.TrimSpace(body[0]) != "")
		if followingNonBlank {
			result = append(result, "")
		}
	}
	result = append(result, afterHeader...)
	result = append(result, body...)

	content := strings.Join(result, "\n") + "\n"
	if err := fileutil.WriteAtomic(path, []byte(content), stat.Mode()); err != nil {
		return fmt.Errorf("annotation: write: %w", err)
	}
	return nil
}
