// Copyright (c) 2026 Visvasity LLC

package subcmds

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/visvasity/hostcheck/linuxcheck"
	"github.com/visvasity/hostcheck/report"
)

// State file names under the data directory.
const (
	baselineFile = "baseline.json"
	currentFile  = "current.json"
)

func baselinePath(dataDir string) string { return filepath.Join(dataDir, baselineFile) }
func currentPath(dataDir string) string  { return filepath.Join(dataDir, currentFile) }

// atomicWriteJSON writes v as indented JSON to path via a temp file + rename, so
// a reader never observes a partial file. The file is mode 0600.
func atomicWriteJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// loadReport reads a report from a JSON file. It returns os.ErrNotExist
// (matchable via errors.Is) when the file is absent.
func loadReport(path string) (*report.Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return decodeReport(data, path)
}

func decodeReport(data []byte, src string) (*report.Report, error) {
	var rep report.Report
	if err := json.Unmarshal(data, &rep); err != nil {
		return nil, fmt.Errorf("parsing report %s: %w", src, err)
	}
	return &rep, nil
}

// loadBaseline loads the configured baseline (data-dir/baseline.json), returning
// os.ErrNotExist when none has been established.
func loadBaseline(dataDir string) (*report.Report, error) {
	return loadReport(baselinePath(dataDir))
}

// collectLocal runs the enabled collectors once via the local agent.
func collectLocal(ctx context.Context, cfg report.Config) *report.Report {
	return linuxcheck.CollectReport(ctx, linuxcheck.DefaultRegistry(), linuxcheck.LocalRunner(), cfg, linuxcheck.Options{})
}

// collectAndRecord collects a report via the local agent and records it as the
// current state (data-dir/current.json), per the requirement to record the
// latest snapshot after a check.
func collectAndRecord(ctx context.Context, dataDir string, cfg report.Config) (*report.Report, error) {
	rep := collectLocal(ctx, cfg)
	if err := atomicWriteJSON(currentPath(dataDir), rep); err != nil {
		return nil, err
	}
	return rep, nil
}

// readReport resolves a report input argument: "" collects a fresh report via
// the local agent, "-" reads JSON from stdin, and anything else reads the named
// JSON file. cfg applies only when collecting a fresh report.
func readReport(ctx context.Context, arg string, cfg report.Config) (*report.Report, error) {
	switch arg {
	case "":
		return collectLocal(ctx, cfg), nil
	case "-":
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("reading report from stdin: %w", err)
		}
		return decodeReport(data, "<stdin>")
	default:
		return loadReport(arg)
	}
}
