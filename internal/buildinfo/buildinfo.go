// Copyright (c) 2026 Visvasity LLC

// Package buildinfo renders the version embedded in the binary by the Go
// toolchain (module version + VCS revision), shared by the module's commands.
package buildinfo

import (
	"runtime/debug"
	"strings"
)

// Version returns a one-line version string derived from build info: the module
// version (from `go install module@version`, "(devel)" for local builds) plus
// the VCS revision/time and a dirty marker when present, and the Go version.
func Version() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return "(unknown: build info unavailable)"
	}
	return format(bi)
}

// format renders a build-info struct; separated from Version so it can be tested
// deterministically with a synthetic *debug.BuildInfo.
func format(bi *debug.BuildInfo) string {
	version := bi.Main.Version
	if version == "" {
		version = "(unknown)"
	}

	var revision, buildTime string
	var modified bool
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.time":
			buildTime = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}

	var b strings.Builder
	b.WriteString(version)
	if revision != "" {
		if len(revision) > 12 {
			revision = revision[:12]
		}
		b.WriteString(" (")
		b.WriteString(revision)
		if modified {
			b.WriteString("-dirty")
		}
		if buildTime != "" {
			b.WriteString(", ")
			b.WriteString(buildTime)
		}
		b.WriteString(")")
	}
	if bi.GoVersion != "" {
		b.WriteString(" ")
		b.WriteString(bi.GoVersion)
	}
	return b.String()
}
