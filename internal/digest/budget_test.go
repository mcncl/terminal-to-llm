package digest

import (
	"strings"
	"testing"
)

func TestEstimateTokens(t *testing.T) {
	// 7 runes / 3.5 = 2 tokens.
	if got := EstimateTokens("abcdefg", 3.5); got != 2 {
		t.Errorf("EstimateTokens = %d, want 2", got)
	}
	// Non-positive falls back to the default divisor.
	if got := EstimateTokens("abcdefg", 0); got != 2 {
		t.Errorf("EstimateTokens fallback = %d, want 2", got)
	}
	if got := EstimateTokens("", 3.5); got != 0 {
		t.Errorf("EstimateTokens empty = %d, want 0", got)
	}
}

func TestBudgetNoOpWhenUnlimited(t *testing.T) {
	lines := []string{"a", "b", "c"}
	got := budget(lines, Options{MaxTokens: 0})
	if strings.Join(got, "\n") != strings.Join(lines, "\n") {
		t.Errorf("budget() unlimited = %v, want unchanged", got)
	}
}

func TestBudgetNoOpWhenUnderTarget(t *testing.T) {
	lines := []string{"short", "lines"}
	got := budget(lines, Options{MaxTokens: 1000, CharsPerToken: 3.5})
	if strings.Join(got, "\n") != strings.Join(lines, "\n") {
		t.Errorf("budget() under target = %v, want unchanged", got)
	}
}

func TestBudgetProtectsFailuresAndDropsNoise(t *testing.T) {
	lines := []string{
		"setup noise alpha bravo charlie delta",
		"setup noise echo foxtrot golf hotel",
		"setup noise india juliet kilo lima",
		"the build failed with an error here",
		"setup noise mike november oscar papa",
	}
	got := budget(lines, Options{MaxTokens: 40, CharsPerToken: 3.5})
	joined := strings.Join(got, "\n")

	if !strings.Contains(joined, "the build failed with an error here") {
		t.Errorf("budget() dropped the failure line: %q", joined)
	}
	if !strings.Contains(joined, "lines omitted") && !strings.Contains(joined, "token budget") {
		t.Errorf("budget() did not annotate trimming: %q", joined)
	}
	if EstimateTokens(joined, 3.5) >= EstimateTokens(strings.Join(lines, "\n"), 3.5) {
		t.Errorf("budget() did not reduce tokens: %q", joined)
	}
}

// TestBudgetAccountsForVerbatimReadds guards against a regression where the
// budget under-counted output because dropping scattered, isolated lines does
// not actually remove them — runs shorter than minOmit are re-emitted verbatim.
// Here every other line is high-value, forcing isolated single-line drops.
func TestBudgetAccountsForVerbatimReadds(t *testing.T) {
	var lines []string
	for i := 0; i < 40; i++ {
		lines = append(lines,
			"error important line that must be kept for sure",
			"low value filler noise line to be dropped away",
		)
	}
	const maxTokens = 200
	got := budget(lines, Options{MaxTokens: maxTokens, CharsPerToken: 3.5})
	joined := strings.Join(got, "\n")
	if n := EstimateTokens(joined, 3.5); n > maxTokens {
		t.Errorf("budget() produced ~%d tokens, over the %d ceiling", n, maxTokens)
	}
}

func TestBudgetKeepsGroupHeaders(t *testing.T) {
	lines := []string{
		"~~~ Section one",
		"noise alpha bravo charlie delta echo one",
		"noise foxtrot golf hotel india juliet two",
		"noise kilo lima mike november oscar three",
		"noise papa quebec romeo sierra tango four",
		"+++ Section two",
		"noise uniform victor whiskey xray yankee five",
		"noise zulu able baker charlie dog easy six",
		"noise fox george how item jig king seven",
		"noise love mike nan oboe peter queen eight",
	}
	got := budget(lines, Options{MaxTokens: 80, CharsPerToken: 3.5})
	joined := strings.Join(got, "\n")
	for _, h := range []string{"~~~ Section one", "+++ Section two"} {
		if !strings.Contains(joined, h) {
			t.Errorf("budget() dropped header %q: %s", h, joined)
		}
	}
}

func TestBudgetTruncatesWhenProtectedExceedsBudget(t *testing.T) {
	// Every line is "important", so none can be dropped normally; the
	// head+tail fallback must still bring it under budget.
	lines := make([]string, 50)
	for i := range lines {
		lines[i] = "error number with some extra padding text here"
	}
	got := budget(lines, Options{MaxTokens: 100, CharsPerToken: 3.5})
	joined := strings.Join(got, "\n")

	if EstimateTokens(joined, 3.5) > 100 {
		t.Errorf("budget() exceeded hard cap: ~%d tokens", EstimateTokens(joined, 3.5))
	}
	if !strings.Contains(joined, "lines omitted") {
		t.Errorf("budget() did not truncate: %q", joined)
	}
}

func TestImportantDistance(t *testing.T) {
	lines := []string{"a", "b", "error here", "c", "d"}
	got := importantDistance(lines)
	want := []int{2, 1, 0, 1, 2}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("importantDistance[%d] = %d, want %d", i, got[i], want[i])
		}
	}

	none := importantDistance([]string{"a", "b"})
	if none[0] != -1 || none[1] != -1 {
		t.Errorf("importantDistance with no failures = %v, want all -1", none)
	}
}
