// Copyright (c) 2026 Visvasity LLC

// Package subcmds implements the gateway server's command-line subcommands on
// top of github.com/visvasity/cli.
package subcmds

import (
	"context"
	"flag"
	"fmt"

	"github.com/visvasity/cli"
	"github.com/visvasity/hostcheck/internal/buildinfo"
)

// VersionCmd prints the version embedded in the binary by the Go toolchain.
type VersionCmd struct{}

func (c *VersionCmd) Purpose() string { return "Print version and build information" }

func (c *VersionCmd) Command() (string, *flag.FlagSet, cli.CmdFunc) {
	return "version", flag.NewFlagSet("version", flag.ContinueOnError), c.run
}

func (c *VersionCmd) run(ctx context.Context, args []string) error {
	fmt.Printf("gateway %s\n", buildinfo.Version())
	return nil
}
