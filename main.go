// Copyright (c) 2026 Visvasity LLC

// Command hostcheck is the HostCheck agent. It collects a host's
// security-relevant state and (in later milestones) diffs it against a baseline
// and reports changes. Install with:
//
//	go install github.com/visvasity/hostcheck@latest
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"

	"github.com/visvasity/cli"
	"github.com/visvasity/hostcheck/subcmds"
	"github.com/visvasity/runcmd"
)

// Exit codes: 0 = success/no change, 1 = changes detected (check), 2 = error.
func main() {
	cmds := []cli.Command{
		new(subcmds.CheckCmd),
		new(subcmds.DiffCmd),
		new(subcmds.AcceptCmd),
		new(subcmds.CollectCmd),
		new(subcmds.ExplainCmd),
		new(subcmds.VersionCmd),
		runcmd.Wrap(new(subcmds.ServeCmd)),
	}
	err := cli.Run(context.Background(), cmds, os.Args[1:])
	if err == nil {
		return
	}
	if errors.Is(err, subcmds.ErrChanged) {
		os.Exit(1) // changes detected — not an operational failure
	}
	slog.Error("failed", "err", err)
	os.Exit(2)
}
