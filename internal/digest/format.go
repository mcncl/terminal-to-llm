package digest

import (
	"regexp"
	"strings"
)

// Format selects how the processed log is rendered.
type Format int

const (
	// FormatPlain renders the log as plain text (the default).
	FormatPlain Format = iota
	// FormatMarkdown renders Buildkite groups as headings and log bodies as
	// fenced code blocks, which gives an LLM a clearer document outline.
	FormatMarkdown
)

// ParseFormat maps a format name to a Format, defaulting to FormatPlain.
func ParseFormat(name string) Format {
	if strings.EqualFold(name, "markdown") {
		return FormatMarkdown
	}
	return FormatPlain
}

// headerPrefixRe matches a leading Buildkite group marker so it can be stripped
// to recover the section title.
var headerPrefixRe = regexp.MustCompile(`^(?:---|\+\+\+|~~~|\^\^\^)\s*`)

// markerRe matches the omission and budget annotations this package emits.
var markerRe = regexp.MustCompile(`^\[… `)

// renderMarkdown turns processed log lines into Markdown: group markers become
// "##" headings, our annotations become italic notes, and runs of ordinary
// lines are wrapped in fenced code blocks.
func renderMarkdown(lines []string) string {
	var b strings.Builder
	body := make([]string, 0, len(lines))

	flush := func() {
		if len(body) == 0 {
			return
		}
		b.WriteString("```\n")
		for _, l := range body {
			b.WriteString(l)
			b.WriteByte('\n')
		}
		b.WriteString("```\n")
		body = body[:0]
	}

	for _, line := range lines {
		switch {
		case groupHeaderRe.MatchString(line):
			flush()
			if title := headingTitle(line); title != "" {
				b.WriteString("## ")
				b.WriteString(title)
				b.WriteByte('\n')
			}
		case markerRe.MatchString(line):
			flush()
			inner := strings.TrimSuffix(strings.TrimPrefix(line, "["), "]")
			b.WriteByte('_')
			b.WriteString(strings.TrimSpace(inner))
			b.WriteString("_\n")
		default:
			body = append(body, line)
		}
	}
	flush()

	return strings.TrimRight(b.String(), "\n")
}

// headingTitle strips leading group markers from a line, returning the section
// title (empty for marker-only lines such as "^^^ +++").
func headingTitle(line string) string {
	for headerPrefixRe.MatchString(line) {
		line = headerPrefixRe.ReplaceAllString(line, "")
	}
	return strings.TrimSpace(line)
}
