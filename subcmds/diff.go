// Copyright (c) 2026 Visvasity LLC

package subcmds

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/visvasity/appdirs"
	"github.com/visvasity/cli"
	"github.com/visvasity/hostcheck/report"
)

// DiffCmd compares a current report against a baseline, read-only. The current
// report is a file path, "-" for stdin, or (default) a fresh local-agent scan.
// The baseline is -baseline PATH, or (default) the configured baseline; it is an
// error if neither exists. Exits non-zero on change, like check, but writes no
// state.
//
//	hostcheck diff                        # fresh scan vs configured baseline
//	hostcheck diff report.json            # given report vs configured baseline
//	hostcheck diff -baseline b.json r.json
//	hostcheck collect | hostcheck diff -
type DiffCmd struct {
	dirs         appdirs.Config
	baselineFile string
	enable       string
	disable      string
}

func (c *DiffCmd) Purpose() string {
	return "Compare a report against a baseline without modifying state"
}

func (c *DiffCmd) Command() (string, *flag.FlagSet, cli.CmdFunc) {
	c.dirs.Program = "hostcheck"
	fset := new(flag.FlagSet)
	c.dirs.SetFlags(fset, &c.dirs)
	fset.StringVar(&c.baselineFile, "baseline", "", "baseline report path (default: the configured baseline)")
	fset.StringVar(&c.enable, "enable", "", "comma-separated module keys to force on (fresh scan only)")
	fset.StringVar(&c.disable, "disable", "", "comma-separated module keys to force off (fresh scan only)")
	return "diff", fset, c.run
}

func (c *DiffCmd) run(ctx context.Context, args []string) error {
	c.dirs.Program = "hostcheck"
	if err := c.dirs.Check(ctx); err != nil {
		return err
	}

	base, err := c.loadBaselineReport()
	if err != nil {
		return err
	}

	var input string
	if len(args) > 0 {
		input = args[0]
	}
	cur, err := readReport(ctx, input, configFrom(c.enable, c.disable))
	if err != nil {
		return err
	}

	d := report.Diff(base, cur)
	if !d.Changed() && len(d.Coverage) == 0 {
		fmt.Fprintln(cli.Stdout(ctx), "no changes since baseline")
		return nil
	}
	renderDiff(cli.Stdout(ctx), d)
	if d.Changed() {
		return ErrChanged
	}
	return nil
}

// loadBaselineReport resolves the baseline from -baseline, else the configured
// baseline, returning a clear error when neither is available.
func (c *DiffCmd) loadBaselineReport() (*report.Report, error) {
	if c.baselineFile != "" {
		return loadReport(c.baselineFile)
	}
	base, err := loadBaseline(c.dirs.DataDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("no baseline: pass -baseline PATH or establish one with `hostcheck accept`")
	}
	return base, err
}
