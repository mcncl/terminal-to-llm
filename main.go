// Command terminal-to-llm converts raw CI/terminal job logs into a compact,
// plain-text form that is cheaper and clearer for an LLM to consume.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/alecthomas/kong"
	"github.com/buildkite/terminal-to-llm/internal/digest"
)

var version = "dev"

// CLI defines the command-line interface.
type CLI struct {
	File string `arg:"" optional:"" type:"existingfile" help:"Log file to read. Reads stdin when omitted."`

	KeepTimestamps bool `help:"Keep leading textual timestamps on each line."`
	KeepDuplicates bool `help:"Do not collapse runs of identical lines."`
	KeepProgress   bool `help:"Do not collapse runs of progress lines (e.g. 12%, 25%)."`
	KeepBlankLines bool `help:"Do not collapse runs of blank lines."`

	NoWindow bool `help:"Disable failure-focused windowing (keep all lines)."`
	Context  int  `default:"15" help:"Lines of context to keep around each important line when windowing."`

	Version kong.VersionFlag `help:"Print the version and exit."`
}

func main() {
	var cli CLI
	kctx := kong.Parse(&cli,
		kong.Name("terminal-to-llm"),
		kong.Description("Digest raw terminal/CI job logs into a compact form for LLMs."),
		kong.UsageOnError(),
		kong.Vars{"version": version},
	)
	kctx.FatalIfErrorf(run(cli))
}

func run(cli CLI) error {
	input, err := readInput(cli.File)
	if err != nil {
		return err
	}

	opt := digest.Options{
		StripTimestamps:    !cli.KeepTimestamps,
		CollapseDuplicates: !cli.KeepDuplicates,
		CollapseProgress:   !cli.KeepProgress,
		TrimBlankRuns:      !cli.KeepBlankLines,
		Window:             !cli.NoWindow,
		ContextLines:       cli.Context,
	}

	out := digest.Process(input, opt)
	if _, err := fmt.Fprintln(os.Stdout, out); err != nil {
		return err
	}
	return nil
}

// readInput reads the named file, or stdin when path is empty.
func readInput(path string) ([]byte, error) {
	if path == "" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(path)
}
