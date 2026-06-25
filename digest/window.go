package digest

import (
	"fmt"
	"regexp"
)

// importantRe matches lines that signal a failure or other content an LLM is
// most likely asked to reason about. It is deliberately broad: over-keeping a
// few benign lines is cheaper than dropping the cause of a failure.
var importantRe = regexp.MustCompile(
	`(?i)(error|fail|fatal|panic|exception|traceback|undefined:|cannot use|exit(?:ed with)? status [1-9])|🚨`,
)

// groupHeaderRe matches Buildkite log group markers, which we always keep as
// structural anchors even when surrounding lines are omitted.
var groupHeaderRe = regexp.MustCompile(`^(?:---|\+\+\+|~~~|\^\^\^)`)

// minOmit is the smallest run of dropped lines worth replacing with a marker;
// shorter gaps are kept verbatim since the marker would not save anything.
const minOmit = 3

// window keeps a context window of ContextLines around each important line,
// always retaining group headers, and replaces longer dropped runs with an
// "[… N lines omitted …]" marker. When no important lines are present the input
// is returned unchanged, so clean logs are never truncated.
func window(lines []string, opt Options) []string {
	if !opt.Window {
		return lines
	}

	keep := make([]bool, len(lines))
	any := false
	for i, line := range lines {
		if importantRe.MatchString(line) {
			any = true
			lo := max(0, i-opt.ContextLines)
			hi := min(len(lines)-1, i+opt.ContextLines)
			for j := lo; j <= hi; j++ {
				keep[j] = true
			}
		}
	}
	if !any {
		return lines
	}
	for i, line := range lines {
		if groupHeaderRe.MatchString(line) {
			keep[i] = true
		}
	}

	return renderWithOmissions(lines, keep)
}

// renderWithOmissions returns lines for which keep is true, replacing each
// dropped run of at least minOmit lines with an "[… N lines omitted …]" marker.
// Shorter dropped runs are emitted verbatim, since a marker would not save space.
func renderWithOmissions(lines []string, keep []bool) []string {
	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); {
		if keep[i] {
			out = append(out, lines[i])
			i++
			continue
		}
		j := i
		for j < len(lines) && !keep[j] {
			j++
		}
		if j-i < minOmit {
			out = append(out, lines[i:j]...)
		} else {
			out = append(out, fmt.Sprintf("[… %d lines omitted …]", j-i))
		}
		i = j
	}
	return out
}
