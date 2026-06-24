// Package digest converts raw CI/terminal job logs into a compact, plain-text
// form that is cheaper and clearer for a large language model to consume.
//
// The pipeline strips ANSI escape sequences and timestamps, resolves
// carriage-return redraws (progress bars, spinners), and collapses runs of
// duplicate or near-duplicate "progress" lines.
package digest

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Options controls which transformations the digest pipeline applies. The
// zero value disables everything; use Default for sensible behaviour.
type Options struct {
	// StripTimestamps removes leading textual timestamps from each line.
	// (ANSI/APC timestamps are always removed as part of escape stripping.)
	StripTimestamps bool
	// CollapseDuplicates folds runs of identical consecutive lines into one
	// line annotated with a "(×N)" count.
	CollapseDuplicates bool
	// CollapseProgress folds runs of lines that differ only by numbers
	// (e.g. "12%", "25%") into the final line plus a count.
	CollapseProgress bool
	// TrimBlankRuns collapses runs of blank lines into a single blank line.
	TrimBlankRuns bool
	// Window keeps only a context window around important (failure) lines,
	// omitting unrelated bulk. It is a no-op when no important lines exist.
	Window bool
	// ContextLines is the number of lines kept on each side of an important
	// line when Window is enabled.
	ContextLines int
	// MaxTokens is a hard ceiling on the estimated token count of the output.
	// Zero means unlimited. When exceeded, the lowest-value lines are dropped
	// first, protecting failure lines and group headers.
	MaxTokens int
	// CharsPerToken is the divisor used to estimate tokens from character
	// count. Lower values are more conservative (over-count tokens).
	CharsPerToken float64
}

// defaultCharsPerToken approximates tokens-per-character for CI/log text
// (paths, hashes and punctuation tokenize more densely than prose) across
// current frontier and open-weight models. It is intentionally conservative.
const defaultCharsPerToken = 3.5

// Default returns Options with every transformation enabled.
func Default() Options {
	return Options{
		StripTimestamps:    true,
		CollapseDuplicates: true,
		CollapseProgress:   true,
		TrimBlankRuns:      true,
		Window:             true,
		ContextLines:       15,
		MaxTokens:          0,
		CharsPerToken:      defaultCharsPerToken,
	}
}

// EstimateTokens approximates the number of tokens in s using charsPerToken.
// A non-positive charsPerToken falls back to defaultCharsPerToken.
func EstimateTokens(s string, charsPerToken float64) int {
	if charsPerToken <= 0 {
		charsPerToken = defaultCharsPerToken
	}
	return int(math.Ceil(float64(utf8.RuneCountInString(s)) / charsPerToken))
}

var (
	digitRe = regexp.MustCompile(`\d+`)
	wsRe    = regexp.MustCompile(`\s+`)
)

// Process runs the full digest pipeline over input and returns the cleaned text.
func Process(input []byte, opt Options) string {
	raw := strings.Split(string(input), "\n")
	lines := make([]string, 0, len(raw))

	for _, line := range raw {
		// Strip escapes before resolving carriage returns: otherwise the
		// column math can slice an escape sequence (e.g. a Buildkite
		// "\x1b_bk;t=...\x07" timestamp) in half, leaking fragments.
		line = stripANSI(line)
		line = resolveCR(line)
		line = strings.TrimRight(line, " \t")
		if opt.StripTimestamps {
			line = stripLeadingTimestamp(line)
		}
		lines = append(lines, line)
	}

	if opt.TrimBlankRuns {
		lines = trimBlankRuns(lines)
	}
	lines = collapse(lines, opt)
	lines = window(lines, opt)
	lines = budget(lines, opt)

	return strings.Join(lines, "\n")
}

// trimBlankRuns collapses consecutive blank lines into a single blank line and
// removes leading/trailing blank lines.
func trimBlankRuns(lines []string) []string {
	out := make([]string, 0, len(lines))
	blank := false
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			blank = true
			continue
		}
		if blank && len(out) > 0 {
			out = append(out, "")
		}
		blank = false
		out = append(out, line)
	}
	return out
}

// normalize returns a comparison key for a line: blank lines map to "" (never
// collapsed), other lines have digit runs replaced with "0" and whitespace
// flattened, so that progress lines such as "12%" and "25%" share a key.
func normalize(s string) string {
	t := strings.TrimSpace(s)
	if t == "" {
		return ""
	}
	t = digitRe.ReplaceAllString(t, "0")
	return wsRe.ReplaceAllString(t, " ")
}

// collapse folds consecutive runs of lines that share a normalized key.
func collapse(lines []string, opt Options) []string {
	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); {
		key := normalize(lines[i])
		j := i + 1
		if key != "" {
			for j < len(lines) && normalize(lines[j]) == key {
				j++
			}
		}
		run := lines[i:j]

		switch {
		case len(run) == 1:
			out = append(out, run[0])
		case identical(run) && opt.CollapseDuplicates:
			out = append(out, fmt.Sprintf("%s  (×%d)", run[0], len(run)))
		case !identical(run) && opt.CollapseProgress:
			out = append(out, fmt.Sprintf("%s  (… %d progress updates collapsed)", run[len(run)-1], len(run)))
		default:
			out = append(out, run...)
		}
		i = j
	}
	return out
}

// identical reports whether every line in run is byte-for-byte equal.
func identical(run []string) bool {
	for _, line := range run[1:] {
		if line != run[0] {
			return false
		}
	}
	return true
}
