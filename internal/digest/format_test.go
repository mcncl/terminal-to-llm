package digest

import "testing"

func TestParseFormat(t *testing.T) {
	cases := map[string]Format{
		"markdown": FormatMarkdown,
		"Markdown": FormatMarkdown,
		"plain":    FormatPlain,
		"":         FormatPlain,
		"nonsense": FormatPlain,
	}
	for in, want := range cases {
		if got := ParseFormat(in); got != want {
			t.Errorf("ParseFormat(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestHeadingTitle(t *testing.T) {
	cases := map[string]string{
		"~~~ Preparing secrets": "Preparing secrets",
		"+++ :hammer: Building": ":hammer: Building",
		"--- Section":           "Section",
		"^^^ +++":               "",
		"~~~":                   "",
		"not a header":          "not a header",
	}
	for in, want := range cases {
		if got := headingTitle(in); got != want {
			t.Errorf("headingTitle(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRenderMarkdown(t *testing.T) {
	lines := []string{
		"~~~ Running commands",
		"$ go test ./...",
		"ok",
		"[… 3 lines omitted …]",
		"^^^ +++",
		"FAIL",
	}
	want := "## Running commands\n" +
		"```\n$ go test ./...\nok\n```\n" +
		"_… 3 lines omitted …_\n" +
		"```\nFAIL\n```"
	if got := renderMarkdown(lines); got != want {
		t.Errorf("renderMarkdown() =\n%q\nwant\n%q", got, want)
	}
}

func TestProcessMarkdownFormat(t *testing.T) {
	opt := Default()
	opt.Format = FormatMarkdown
	in := []byte("~~~ Build\nhello\nworld")
	want := "## Build\n```\nhello\nworld\n```"
	if got := Process(in, opt); got != want {
		t.Errorf("Process(markdown) =\n%q\nwant\n%q", got, want)
	}
}
