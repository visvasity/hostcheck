// Copyright (c) 2026 Visvasity LLC

package subcmds

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/visvasity/appdirs"
	"github.com/visvasity/cli"
	"github.com/visvasity/hostcheck/report"
	"github.com/visvasity/logdir"
	"github.com/visvasity/runcmd"
)

// ServeCmd runs the host-check agent as a service: on a fixed interval it
// collects a report, records it as the current state, and diffs it against the
// baseline, logging any changes. Wrapped by runcmd.Wrap, it gains -background,
// -restart, and -self-monitor for systemd/daemon use.
//
// The baseline is never auto-established: until an operator runs `hostcheck
// accept`, change detection is inactive (the service still records the current
// snapshot). Alerting and upload are added in later milestones.
type ServeCmd struct {
	dirs appdirs.Config

	interval  time.Duration
	logDebug  bool
	logStderr bool
}

func (c *ServeCmd) Purpose() string {
	return "Run the host-check agent as a periodic service"
}

func (c *ServeCmd) Command() (string, *flag.FlagSet, cli.CmdFunc) {
	c.dirs.Program = "hostcheck"
	fset := new(flag.FlagSet)
	c.dirs.SetFlags(fset, &c.dirs)
	fset.DurationVar(&c.interval, "interval", 15*time.Minute, "Interval between collections")
	fset.BoolVar(&c.logDebug, "log-debug", false, "When true, enables debug logging")
	fset.BoolVar(&c.logStderr, "logtostderr", false, "When true, logs are written only to stderr")
	return "run", fset, c.run
}

// LocksDir tells runcmd where to place its daemon lock/socket files.
func (c *ServeCmd) LocksDir() string {
	return c.dirs.RuntimeDir
}

func (c *ServeCmd) Check(ctx context.Context) error {
	c.dirs.Program = "hostcheck"
	if err := c.dirs.Check(ctx); err != nil {
		return err
	}
	if c.interval <= 0 {
		return fmt.Errorf("interval (-interval) must be positive")
	}
	if r, ok := runcmd.FromContext(ctx); ok && r.Background && c.logStderr {
		return fmt.Errorf("logging to stderr (-logtostderr) cannot be used with -background")
	}
	return nil
}

func (c *ServeCmd) run(ctx context.Context, args []string) error {
	if err := c.Check(ctx); err != nil {
		return err
	}

	level := slog.LevelInfo
	if c.logDebug {
		level = slog.LevelDebug
	}
	slog.SetLogLoggerLevel(level)
	if !c.logStderr {
		sink, err := logdir.Open(logdir.Config{Dir: c.dirs.LogDir, Level: level})
		if err != nil {
			return err
		}
		slog.SetDefault(sink.Logger(""))
	}

	check := func() {
		cur, err := collectAndRecord(ctx, c.dirs.DataDir, report.Config{})
		if err != nil {
			slog.Error("collect failed", "err", err)
			return
		}
		base, err := loadBaseline(c.dirs.DataDir)
		if errors.Is(err, os.ErrNotExist) {
			// The baseline is never auto-established; it must be an explicit
			// operator action. Until then, change detection is inactive (the
			// current snapshot is still recorded for `accept` and inspection).
			slog.Warn("no baseline established; change detection inactive — run `hostcheck accept` to establish one", "data_dir", c.dirs.DataDir)
			return
		}
		if err != nil {
			slog.Error("could not load baseline", "err", err)
			return
		}
		d := report.Diff(base, cur)
		if d.Changed() {
			slog.Warn("changes detected since baseline", "incidents", len(d.Incidents), "coverage", len(d.Coverage))
			for _, ch := range d.Incidents {
				slog.Warn("change", "path", ch.Path, "kind", string(ch.Kind), "old", ch.Old, "new", ch.New)
			}
		} else {
			slog.Info("no changes since baseline")
		}
	}

	// First check doubles as the initialization step; report its outcome to the
	// foreground/monitor process so -background can succeed or fail fast.
	check()
	runcmd.Report(ctx, nil)

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	slog.Info("host-check agent started", "interval", c.interval, "data_dir", c.dirs.DataDir)
	for {
		select {
		case <-ctx.Done():
			slog.Info("host-check agent stopping", "cause", context.Cause(ctx))
			return nil
		case <-ticker.C:
			check()
		}
	}
}
