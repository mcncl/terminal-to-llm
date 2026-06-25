package digest

import (
	"regexp"
	"strings"
)

// ansiRe matches ANSI/VT escape sequences we want to discard entirely.
//
// It covers, in order:
//   - CSI sequences:        ESC [ ... final-byte   (colours, cursor moves)
//   - OSC sequences:        ESC ] ... (BEL | ST)   (window titles, hyperlinks)
//   - APC/DCS/SOS/PM:       ESC (_/P/X/^) ... (BEL | ST)
//     Buildkite embeds per-line timestamps as APC sequences: ESC _ bk;t=<ms> BEL
//   - lone two-byte escapes: ESC <single byte>
var ansiRe = regexp.MustCompile(
	`\x1b\[[0-9;?]*[ -/]*[@-~]` +
		`|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)` +
		`|\x1b[_PX^][^\x1b\x07]*(?:\x07|\x1b\\)` +
		`|\x1b[@-Z\\-_]`,
)

// controlRe matches leftover control characters that carry no useful text,
// preserving tab and newline (carriage returns are handled separately).
var controlRe = regexp.MustCompile("[\x00-\x08\x0b\x0c\x0e-\x1f\x7f]")

// stripANSI removes ANSI escape sequences and stray control characters.
func stripANSI(s string) string {
	s = ansiRe.ReplaceAllString(s, "")
	return controlRe.ReplaceAllString(s, "")
}

// resolveCR collapses carriage-return overwrites within a single line,
// emulating a terminal cursor returning to column zero. Progress bars and
// spinners redraw the same line many times via "\r"; we keep only the final
// rendered state.
func resolveCR(line string) string {
	if !strings.ContainsRune(line, '\r') {
		return line
	}
	var buf []rune
	col := 0
	for _, r := range line {
		if r == '\r' {
			col = 0
			continue
		}
		if col < len(buf) {
			buf[col] = r
		} else {
			buf = append(buf, r)
		}
		col++
	}
	return string(buf)
}

// timestampRe matches a leading textual timestamp such as
// "2024-01-02T15:04:05.123Z ", "[2024-01-02 15:04:05] " or "15:04:05.123 ".
var timestampRe = regexp.MustCompile(
	`^\s*\[?(?:\d{4}-\d{2}-\d{2}[T ])?\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})?\]?\s+`,
)

// stripLeadingTimestamp removes a textual timestamp prefix from a line.
func stripLeadingTimestamp(line string) string {
	return timestampRe.ReplaceAllString(line, "")
}
