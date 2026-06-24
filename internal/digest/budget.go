package digest

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// budgetHeadroom targets a fraction of MaxTokens so that approximation error
// and the marker lines we add do not push the result over the real ceiling.
const budgetHeadroom = 0.95

// cmdRe matches shell command lines, including Buildkite plugin-prefixed ones
// such as "[vulncheck] $ govulncheck ./...". These are kept preferentially.
var cmdRe = regexp.MustCompile(`^(?:\[[^\]]*\] )?\$ `)

// budget enforces a hard token ceiling on the output by dropping the
// lowest-value lines first. Value is scored so that failure lines and group
// headers far outrank boilerplate; only when the budget is too small to hold
// even those are the least valuable of them dropped. It is a no-op when
// MaxTokens is zero.
func budget(lines []string, opt Options) []string {
	if opt.MaxTokens <= 0 || len(lines) == 0 {
		return lines
	}

	target := int(float64(opt.MaxTokens) * budgetHeadroom)
	if target < 1 {
		target = 1
	}

	cost := make([]int, len(lines))
	total := 0
	for i, line := range lines {
		cost[i] = EstimateTokens(line, opt.CharsPerToken)
		total += cost[i]
	}
	if total <= target {
		return lines
	}

	// Reserve room for the footer and a representative omission marker so the
	// annotations we add do not themselves push the output over the ceiling.
	footer := fmt.Sprintf("[… trimmed to fit ~%d token budget …]", opt.MaxTokens)
	reserve := EstimateTokens(footer, opt.CharsPerToken) +
		EstimateTokens("[… 0000 lines omitted …]", opt.CharsPerToken)
	if r := target - reserve; r >= 1 {
		target = r
	} else {
		target = 1
	}

	keep := make([]bool, len(lines))
	for i := range keep {
		keep[i] = true
	}
	for _, i := range dropOrder(lines) {
		if total <= target {
			break
		}
		keep[i] = false
		total -= cost[i]
	}

	out := renderWithOmissions(lines, keep)
	return append(out, footer)
}

// dropOrder returns every line index ordered from least to most valuable, so
// callers can drop from the front. Ties prefer dropping earlier lines, biasing
// retention toward the end of the log where the failure cause usually sits.
func dropOrder(lines []string) []int {
	dist := importantDistance(lines)
	n := len(lines)

	type cand struct{ idx, score int }
	cands := make([]cand, n)
	for i, line := range lines {
		cands[i] = cand{i, lineScore(line, i, n, dist[i])}
	}
	sort.Slice(cands, func(a, b int) bool {
		if cands[a].score != cands[b].score {
			return cands[a].score < cands[b].score
		}
		return cands[a].idx < cands[b].idx
	})

	out := make([]int, n)
	for i, c := range cands {
		out[i] = c.idx
	}
	return out
}

// lineScore rates a line's value for retention: higher is more valuable.
func lineScore(line string, idx, n, dist int) int {
	if strings.TrimSpace(line) == "" {
		return 0
	}
	score := 10
	switch {
	case importantRe.MatchString(line):
		score += 100 // failure cause: keep above all else
	case groupHeaderRe.MatchString(line):
		score += 80 // structural anchor
	}
	if cmdRe.MatchString(line) {
		score += 40
	}
	if dist >= 0 {
		score += max(0, 40-dist) // proximity to a failure line
	}
	if n > 1 {
		score += 20 * idx / (n - 1) // tail bias
	}
	return score
}

// importantDistance returns, for each line, the distance to the nearest
// important line, or -1 when there are no important lines.
func importantDistance(lines []string) []int {
	const inf = 1 << 30
	n := len(lines)
	dist := make([]int, n)
	for i := range dist {
		dist[i] = inf
	}

	prev := -1
	for i, line := range lines {
		if importantRe.MatchString(line) {
			prev = i
		}
		if prev >= 0 {
			dist[i] = i - prev
		}
	}
	next := -1
	for i := n - 1; i >= 0; i-- {
		if importantRe.MatchString(lines[i]) {
			next = i
		}
		if next >= 0 && next-i < dist[i] {
			dist[i] = next - i
		}
	}
	for i := range dist {
		if dist[i] == inf {
			dist[i] = -1
		}
	}
	return dist
}
