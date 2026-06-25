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

	cpt := opt.CharsPerToken
	target := int(float64(opt.MaxTokens) * budgetHeadroom)
	if target < 1 {
		target = 1
	}
	if EstimateTokens(strings.Join(lines, "\n"), cpt) <= target {
		return lines
	}

	n := len(lines)
	footer := fmt.Sprintf("[… trimmed to fit ~%d token budget …]", opt.MaxTokens)

	// total tracks the estimated token count of the rendered output. We drop
	// lines lowest-value first, but a dropped line only saves tokens once its
	// run reaches minOmit and becomes a marker; shorter runs are re-emitted
	// verbatim by renderWithOmissions, so we account for that exactly here.
	cost := make([]int, n)
	total := EstimateTokens(footer, cpt)
	for i, line := range lines {
		cost[i] = EstimateTokens(line, cpt)
		total += cost[i]
	}

	contribution := func(length, sum int) int {
		if length >= minOmit {
			return EstimateTokens(fmt.Sprintf("[… %d lines omitted …]", length), cpt)
		}
		return sum // re-emitted verbatim, so it still costs its full size
	}

	// Per dropped run we store, at its start a: end index and summed cost; at
	// its end b: the start index. This merges adjacent runs in amortised O(1).
	dropped := make([]bool, n)
	runEnd := make([]int, n)
	runStart := make([]int, n)
	runSum := make([]int, n)

	for _, i := range dropOrder(lines) {
		if total <= target {
			break
		}
		total -= cost[i] // no longer counted as a kept line
		a, b, sum := i, i, cost[i]
		if i > 0 && dropped[i-1] {
			la := runStart[i-1]
			total -= contribution(i-la, runSum[la])
			a, sum = la, sum+runSum[la]
		}
		if i < n-1 && dropped[i+1] {
			rb := runEnd[i+1]
			total -= contribution(rb-i, runSum[i+1])
			b, sum = rb, sum+runSum[i+1]
		}
		dropped[i] = true
		runEnd[a], runStart[b], runSum[a] = b, a, sum
		total += contribution(b-a+1, sum)
	}

	keep := make([]bool, n)
	for i := range keep {
		keep[i] = !dropped[i]
	}
	return append(renderWithOmissions(lines, keep), footer)
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
