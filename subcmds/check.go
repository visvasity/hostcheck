// Copyright (c) 2026 Visvasity LLC

package subcmds

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/visvasity/appdirs"
	"github.com/visvasity/cli"
	"github.com/visvasity/hostcheck/report"
)

// ErrChanged is returned by `check` and `diff` when incident-level changes are
// detected. main maps it to a distinct process exit code.
var ErrChanged = errors.New("changes detected since baseline")

// CheckCmd is the operational monitoring cycle: collect via the local agent,
// record the current snapshot, and diff it against the configured baseline. It
// requires a baseline to exist (establish one with `accept`) and never advances
// it. This is the command a cron job or the service loop runs.
type CheckCmd struct {
	dirs    appdirs.Config
	enable  string
	disable string
}

func (c *CheckCmd) Purpose() string {
	return "Collect, diff against the configured baseline, and report changes (exit non-zero on change)"
}

func (c *CheckCmd) Command() (string, *flag.FlagSet, cli.CmdFunc) {
	c.dirs.Program = "hostcheck"
	fset := new(flag.FlagSet)
	c.dirs.SetFlags(fset, &c.dirs)
	fset.StringVar(&c.enable, "enable", "", "comma-separated module keys to force on")
	fset.StringVar(&c.disable, "disable", "", "comma-separated module keys to force off")
	return "check", fset, c.run
}

func (c *CheckCmd) run(ctx context.Context, args []string) error {
	c.dirs.Program = "hostcheck"
	if err := c.dirs.Check(ctx); err != nil {
		return err
	}

	base, err := loadBaseline(c.dirs.DataDir)
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("no baseline established; run `hostcheck accept` to establish one")
	}
	if err != nil {
		return err
	}

	cur, err := collectAndRecord(ctx, c.dirs.DataDir, configFrom(c.enable, c.disable))
	if err != nil {
		return err
	}

	d := report.Diff(base, cur)
	renderDiff(cli.Stdout(ctx), d)
	if d.Changed() {
		return ErrChanged
	}
	fmt.Fprintln(cli.Stdout(ctx), "no changes since baseline")
	return nil
}

// renderDiff writes a human-readable summary of a diff result.
func renderDiff(w io.Writer, d report.DiffResult) {
	if len(d.Incidents) > 0 {
		fmt.Fprintln(w, "changes:")
		for _, c := range d.Incidents {
			fmt.Fprintf(w, "  %s\n", renderChange(c))
		}
	}
	if len(d.Coverage) > 0 {
		fmt.Fprintln(w, "coverage changes:")
		for _, c := range d.Coverage {
			fmt.Fprintf(w, "  %s: %s -> %s\n", c.Path, dash(c.Old), dash(c.New))
		}
	}
}

func renderChange(c report.Change) string {
	switch c.Kind {
	case report.ChangeAdded:
		return "+ " + c.Path + ": " + c.New
	case report.ChangeRemoved:
		return "- " + c.Path + ": " + c.Old
	default:
		return "~ " + c.Path + ": " + c.Old + " -> " + c.New
	}
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
