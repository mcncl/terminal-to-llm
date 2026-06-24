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

// budget enforces a hard token ceiling on the output. It drops the
// lowest-value lines first, never dropping failure lines or group headers
// unless even those alone exceed the budget, in which case it falls back to a
// tail-biased head+tail truncation. It is a no-op when MaxTokens is zero.
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
	protected := make([]bool, len(lines))
	for i, line := range lines {
		keep[i] = true
		protected[i] = importantRe.MatchString(line) || groupHeaderRe.MatchString(line)
	}

	for _, i := range dropOrder(lines, protected) {
		if total <= target {
			break
		}
		keep[i] = false
		total -= cost[i]
	}

	var out []string
	if total > target {
		// Even the protected lines exceed the budget: truncate hard, keeping
		// the head and (preferentially) the tail where the exit status lives.
		out = truncateHeadTail(lines, cost, target)
	} else {
		out = renderWithOmissions(lines, keep)
	}
	return append(out, footer)
}

// dropOrder returns the indices of non-protected lines ordered from least to
// most valuable, so callers can drop from the front. Ties prefer dropping
// earlier lines, biasing retention toward the end of the log.
func dropOrder(lines []string, protected []bool) []int {
	dist := importantDistance(lines)
	n := len(lines)

	type cand struct{ idx, score int }
	cands := make([]cand, 0, n)
	for i, line := range lines {
		if protected[i] {
			continue
		}
		cands = append(cands, cand{i, lineScore(line, i, n, dist[i])})
	}
	sort.Slice(cands, func(a, b int) bool {
		if cands[a].score != cands[b].score {
			return cands[a].score < cands[b].score
		}
		return cands[a].idx < cands[b].idx
	})

	out := make([]int, len(cands))
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

// truncateHeadTail keeps as many tail lines as fit in most of the budget, then
// fills the remainder from the head, dropping the middle. The tail is favoured
// because the failure cause and exit status usually appear at the end.
func truncateHeadTail(lines []string, cost []int, target int) []string {
	n := len(lines)
	keep := make([]bool, n)
	used := 0

	tailBudget := target * 7 / 10
	for i := n - 1; i >= 0; i-- {
		if used+cost[i] > tailBudget {
			break
		}
		keep[i] = true
		used += cost[i]
	}
	for i := 0; i < n; i++ {
		if keep[i] {
			continue
		}
		if used+cost[i] > target {
			break
		}
		keep[i] = true
		used += cost[i]
	}
	return renderWithOmissions(lines, keep)
}
