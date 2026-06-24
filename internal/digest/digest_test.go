package digest

import "testing"

func TestStripANSI(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"color", "\x1b[31mred\x1b[0m", "red"},
		{"cursor", "abc\x1b[2Kdef", "abcdef"},
		{"osc hyperlink", "\x1b]8;;http://x\x07link\x1b]8;;\x07", "link"},
		{"buildkite timestamp", "\x1b_bk;t=1638362886443\x07hello", "hello"},
		{"plain", "nothing here", "nothing here"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := stripANSI(tc.in); got != tc.want {
				t.Errorf("stripANSI(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestResolveCR(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"progress redraw", "10%\r50%\r100%", "100%"},
		{"partial overwrite", "loading...\rdone", "doneing..."},
		{"no cr", "plain", "plain"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveCR(tc.in); got != tc.want {
				t.Errorf("resolveCR(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestStripLeadingTimestamp(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"2024-01-02T15:04:05.123Z building", "building"},
		{"[2024-01-02 15:04:05] step started", "step started"},
		{"15:04:05 running tests", "running tests"},
		{"15:04:05.999 done", "done"},
		{"no timestamp here", "no timestamp here"},
	}
	for _, tc := range tests {
		if got := stripLeadingTimestamp(tc.in); got != tc.want {
			t.Errorf("stripLeadingTimestamp(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestProcessCollapsesDuplicates(t *testing.T) {
	in := []byte("same\nsame\nsame\ndifferent")
	want := "same  (×3)\ndifferent"
	if got := Process(in, Default()); got != want {
		t.Errorf("Process() = %q, want %q", got, want)
	}
}

func TestProcessCollapsesProgress(t *testing.T) {
	in := []byte("Downloading 0%\nDownloading 25%\nDownloading 50%\nDownloading 100%\nDone")
	want := "Downloading 100%  (… 4 progress updates collapsed)\nDone"
	if got := Process(in, Default()); got != want {
		t.Errorf("Process() = %q, want %q", got, want)
	}
}

func TestProcessTrimsBlankRuns(t *testing.T) {
	in := []byte("a\n\n\n\nb")
	want := "a\n\nb"
	if got := Process(in, Default()); got != want {
		t.Errorf("Process() = %q, want %q", got, want)
	}
}

func TestProcessRespectsDisabledOptions(t *testing.T) {
	in := []byte("same\nsame")
	want := "same\nsame"
	if got := Process(in, Options{}); got != want {
		t.Errorf("Process() with zero Options = %q, want %q", got, want)
	}
}

func TestProcessEndToEnd(t *testing.T) {
	in := []byte("\x1b_bk;t=1638362886443\x07\x1b[32m15:04:05 Building\x1b[0m\n" +
		"progress 10%\rprogress 60%\rprogress 100%\n" +
		"ok\nok\nok")
	want := "Building\nprogress 100%\nok  (×3)"
	if got := Process(in, Default()); got != want {
		t.Errorf("Process() = %q, want %q", got, want)
	}
}
