<img style="display: block; margin: 0 auto;" src="header.svg" alt="TERMINAL-TO-LLM" width="700">

Digest raw terminal / CI job logs into a compact, plain-text form that is cheaper and clearer for a large language model to consume.

It is the spiritual opposite of [terminal-to-html](https://github.com/buildkite/terminal-to-html): instead of rendering shell output into *beautiful* HTML, it strips everything an LLM does not need and focuses on what it does — usually, *why a build failed*.

## What it does

The pipeline runs in stages:

```
raw log ─▶ strip ANSI/timestamps ─▶ resolve CR redraws ─▶ collapse dup/progress
        ─▶ trim blank runs ─▶ failure-focused windowing ─▶ token budget ─▶ render
```

### Strips ANSI escapes and timestamps

Colours, cursor moves, OSC sequences, and Buildkite's per-line `\x1b_bk;t=…\x07` timestamps.

### Resolves carriage-return redraws
 
Progress bars and spinners that rewrite one line via `\r` are reduced to their final rendered state.

### Collapses duplicate and progress lines

Identical consecutive lines fold to `… (×N)`; lines differing only by numbers (`12%`, `25%`) fold to the final value plus a count.

### Failure-focused windowing

Keeps a context window around error/failure lines (and always keeps Buildkite `~~~` / `+++` / `^^^` group markers as structure), replacing unrelated bulk with `[… N lines omitted …]`. Clean logs with no failures are left untouched.

### Token budgeting

An optional hard ceiling that drops the lowest-value lines first (failure lines and group headers are scored highest), so the *why* survives even at an aggressive budget.

### Markdown output

Optionally render groups as headings and log bodies as fenced code blocks for a clearer document outline.

## Install

```
go install github.com/mcncl/terminal-to-llm@latest
```

## Library usage

`terminal-to-llm` is also a Go package. Add it to your module:

```
go get github.com/mcncl/terminal-to-llm/digest
```

Then digest a raw log with `digest.Process`, which takes the raw bytes and an
`Options` value and returns the cleaned string:

```go
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/mcncl/terminal-to-llm/digest"
)

func main() {
	// Raw log bytes from anywhere: a file, an HTTP body, `bk job log`, etc.
	rawLog, err := io.ReadAll(os.Stdin)
	if err != nil {
		panic(err)
	}

	// Start from the defaults (every transform on, no token limit) and tweak.
	opt := digest.Default()
	opt.MaxTokens = 2000              // hard ceiling on the output (0 = unlimited)
	opt.Format = digest.FormatMarkdown // or digest.FormatPlain

	out := digest.Process(rawLog, opt)
	fmt.Println(out)
}
```

You can also estimate tokens directly without processing:

```go
n := digest.EstimateTokens(out, 3.5)
```

## Flags

| Flag | Default | Description |
| --- | --- | --- |
| `--keep-timestamps` | off | Keep leading textual timestamps on each line. |
| `--keep-duplicates` | off | Do not collapse runs of identical lines. |
| `--keep-progress` | off | Do not collapse runs of progress lines (e.g. `12%`, `25%`). |
| `--keep-blank-lines` | off | Do not collapse runs of blank lines. |
| `--no-window` | off | Disable failure-focused windowing (keep all lines). |
| `--context` | `15` | Lines of context to keep around each important line when windowing. |
| `--max-tokens` | `0` | Hard ceiling on the estimated tokens of the output (`0` = unlimited). |
| `--chars-per-token` | `3.5` | Characters-per-token used to estimate token counts. Lower is more conservative. |
| `--format` | `plain` | Output format: `plain` or `markdown`. |

### A note on `--max-tokens`

This is the budget for **the log**, not the model's full context window — logs are one input among the system prompt, the question, and other context, so set it to whatever slice you are giving the log.

Token counts are *estimated* from character count (`--chars-per-token`), not a real tokenizer. There is no single offline tokenizer that is accurate across Claude, GPT and open-weight models, and for code/log text they all land in roughly 3.3–4.0 characters per token. The default of `3.5` is deliberately conservative (it slightly over-counts, keeping you under the real limit). For an unusual model you can tune it. The result is a hard cap at realistic budgets and best-effort at very small ones, where fixed marker overhead dominates.
