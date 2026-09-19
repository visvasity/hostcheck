// Copyright (c) 2026 Visvasity LLC

package subcmds

import (
	"context"
	"flag"
	"fmt"

	"github.com/visvasity/appdirs"
	"github.com/visvasity/cli"
)

// AcceptCmd sets the baseline that subsequent checks compare against. It accepts
// a report to adopt as the new baseline: a file path, "-" for stdin, or no
// argument to collect a fresh report via the local agent.
//
//	hostcheck accept                  # baseline = fresh local-agent report
//	hostcheck accept report.json      # baseline = the given report
//	hostcheck collect | hostcheck accept -
type AcceptCmd struct {
	dirs    appdirs.Config
	enable  string
	disable string
}

func (c *AcceptCmd) Purpose() string {
	return "Adopt a report (or a fresh local scan) as the new baseline"
}

func (c *AcceptCmd) Command() (string, *flag.FlagSet, cli.CmdFunc) {
	c.dirs.Program = "hostcheck"
	fset := new(flag.FlagSet)
	c.dirs.SetFlags(fset, &c.dirs)
	fset.StringVar(&c.enable, "enable", "", "comma-separated module keys to force on (fresh scan only)")
	fset.StringVar(&c.disable, "disable", "", "comma-separated module keys to force off (fresh scan only)")
	return "accept", fset, c.run
}

func (c *AcceptCmd) run(ctx context.Context, args []string) error {
	c.dirs.Program = "hostcheck"
	if err := c.dirs.Check(ctx); err != nil {
		return err
	}

	var input string
	if len(args) > 0 {
		input = args[0]
	}
	rep, err := readReport(ctx, input, configFrom(c.enable, c.disable))
	if err != nil {
		return err
	}

	if err := atomicWriteJSON(baselinePath(c.dirs.DataDir), rep); err != nil {
		return err
	}
	fmt.Fprintf(cli.Stdout(ctx), "baseline updated: %s\n", baselinePath(c.dirs.DataDir))
	return nil
}
