// Copyright (c) 2026 Visvasity LLC

// Command gateway is the Visvasity Gateway web-frontend server: it serves the
// subscriber dashboard (GATEWAY.md) and, later, report ingestion. This build is
// scaffolding — the HTTP server, logging, and daemon lifecycle are in place; the
// UI pages are not implemented yet.
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/visvasity/cli"
	"github.com/visvasity/hostcheck/gateway/subcmds"
	"github.com/visvasity/runcmd"
)

func main() {
	cmds := []cli.Command{
		runcmd.Wrap(new(subcmds.ServeCmd)),
		new(subcmds.VersionCmd),
	}
	if err := cli.Run(context.Background(), cmds, os.Args[1:]); err != nil {
		slog.Error("failed", "err", err)
		os.Exit(1)
	}
}
