package digest

import (
	"strings"
	"testing"
)

func TestWindowNoImportantLinesUnchanged(t *testing.T) {
	lines := []string{"a", "b", "c", "d", "e"}
	opt := Options{Window: true, ContextLines: 1}
	got := window(lines, opt)
	if strings.Join(got, "\n") != strings.Join(lines, "\n") {
		t.Errorf("window() = %v, want unchanged %v", got, lines)
	}
}

func TestWindowKeepsContextAroundFailure(t *testing.T) {
	lines := []string{
		"noise 1", "noise 2", "noise 3", "noise 4", "noise 5",
		"build failed here",
		"noise 6", "noise 7", "noise 8", "noise 9", "noise 10",
	}
	opt := Options{Window: true, ContextLines: 1}
	got := window(lines, opt)
	want := []string{
		"[… 4 lines omitted …]",
		"noise 5",
		"build failed here",
		"noise 6",
		"[… 4 lines omitted …]",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("window() = %#v, want %#v", got, want)
	}
}

func TestWindowAlwaysKeepsGroupHeaders(t *testing.T) {
	lines := []string{
		"~~~ Section one",
		"noise 1", "noise 2", "noise 3", "noise 4",
		"+++ Section two",
		"noise 5", "noise 6", "noise 7", "noise 8",
		"panic: boom",
	}
	opt := Options{Window: true, ContextLines: 0}
	got := window(lines, opt)
	want := []string{
		"~~~ Section one",
		"[… 4 lines omitted …]",
		"+++ Section two",
		"[… 4 lines omitted …]",
		"panic: boom",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("window() = %#v, want %#v", got, want)
	}
}

func TestWindowKeepsShortGapsVerbatim(t *testing.T) {
	lines := []string{"error: x", "a", "b", "error: y"}
	opt := Options{Window: true, ContextLines: 0}
	got := window(lines, opt)
	want := []string{"error: x", "a", "b", "error: y"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("window() = %#v, want %#v", got, want)
	}
}

func TestWindowDisabled(t *testing.T) {
	lines := []string{"a", "error", "b", "c", "d", "e", "f"}
	got := window(lines, Options{Window: false})
	if strings.Join(got, "\n") != strings.Join(lines, "\n") {
		t.Errorf("window() disabled = %v, want unchanged", got)
	}
}

func TestImportantPatterns(t *testing.T) {
	match := []string{
		"build ERROR", "tests failed", "panic: nil deref",
		"undefined: foo", "cannot use x as y", "exited with status 1",
		"exit status 2", "🚨 Error", "Traceback (most recent call last):",
	}
	for _, s := range match {
		if !importantRe.MatchString(s) {
			t.Errorf("importantRe should match %q", s)
		}
	}
	noMatch := []string{"all good", "exit status 0", "compiling package"}
	for _, s := range noMatch {
		if importantRe.MatchString(s) {
			t.Errorf("importantRe should not match %q", s)
		}
	}
}
