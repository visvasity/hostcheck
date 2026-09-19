// Copyright (c) 2026 Visvasity LLC

package buildinfo

import (
	"runtime/debug"
	"strings"
	"testing"
)

func TestFormat(t *testing.T) {
	bi := &debug.BuildInfo{
		GoVersion: "go1.26.0",
		Main:      debug.Module{Version: "v1.2.3"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "abcdef1234567890deadbeef"},
			{Key: "vcs.time", Value: "2026-09-19T00:00:00Z"},
			{Key: "vcs.modified", Value: "true"},
		},
	}
	got := format(bi)
	for _, want := range []string{"v1.2.3", "abcdef123456", "-dirty", "2026-09-19T00:00:00Z", "go1.26.0"} {
		if !strings.Contains(got, want) {
			t.Errorf("format = %q, missing %q", got, want)
		}
	}
	if strings.Contains(got, "abcdef1234567890") {
		t.Errorf("revision not truncated to 12 chars: %q", got)
	}
}

func TestFormatDevel(t *testing.T) {
	bi := &debug.BuildInfo{GoVersion: "go1.26.0", Main: debug.Module{Version: "(devel)"}}
	got := format(bi)
	if !strings.Contains(got, "(devel)") || !strings.Contains(got, "go1.26.0") {
		t.Errorf("format = %q", got)
	}
	if strings.Contains(got, "-dirty") {
		t.Errorf("unexpected dirty marker: %q", got)
	}
}

func TestFormatEmptyMain(t *testing.T) {
	bi := &debug.BuildInfo{GoVersion: "go1.26.0"}
	if got := format(bi); !strings.Contains(got, "(unknown)") {
		t.Errorf("empty main version should render (unknown): %q", got)
	}
}
