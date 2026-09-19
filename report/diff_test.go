// Copyright (c) 2026 Visvasity LLC

package report

import (
	"testing"
	"time"
)

func collectedSSHD(pw bool) Section[SSHDConfig] {
	return Section[SSHDConfig]{Status: StatusCollected, Data: &SSHDConfig{PasswordAuthentication: pw}}
}

func collectedListeners(socks ...ListeningSocket) Section[ListeningSockets] {
	return Section[ListeningSockets]{Status: StatusCollected, Data: &ListeningSockets{Sockets: socks}}
}

func TestDiffNoChange(t *testing.T) {
	a := &Report{GeneratedAt: time.Unix(1, 0), SSHDConfig: collectedSSHD(false)}
	b := &Report{GeneratedAt: time.Unix(2, 0), SSHDConfig: collectedSSHD(false)} // only timestamp differs
	d := Diff(a, b)
	if d.Changed() || len(d.Coverage) != 0 {
		t.Errorf("expected no change, got %+v", d)
	}
}

func TestDiffScalarModified(t *testing.T) {
	a := &Report{SSHDConfig: collectedSSHD(false)}
	b := &Report{SSHDConfig: collectedSSHD(true)}
	d := Diff(a, b)
	if !d.Changed() || len(d.Incidents) != 1 {
		t.Fatalf("expected 1 incident, got %+v", d)
	}
	c := d.Incidents[0]
	if c.Path != "sshd_config.password_authentication" || c.Kind != ChangeModified || c.Old != "false" || c.New != "true" {
		t.Errorf("unexpected change: %+v", c)
	}
}

func TestDiffArrayAddRemove(t *testing.T) {
	a := collectedListeners(
		ListeningSocket{Protocol: "tcp", BindClass: "loopback", Port: 5432, Program: "postgres"},
	)
	b := collectedListeners(
		ListeningSocket{Protocol: "tcp", BindClass: "wildcard", Port: 8080, Program: "nginx"},
	)
	d := Diff(&Report{ListeningTCP: a}, &Report{ListeningTCP: b})
	if len(d.Incidents) != 2 {
		t.Fatalf("expected add+remove = 2 incidents, got %+v", d.Incidents)
	}
	var added, removed int
	for _, c := range d.Incidents {
		if c.Path != "listening_tcp.sockets" {
			t.Errorf("unexpected path %q", c.Path)
		}
		switch c.Kind {
		case ChangeAdded:
			added++
		case ChangeRemoved:
			removed++
		}
	}
	if added != 1 || removed != 1 {
		t.Errorf("added=%d removed=%d, want 1/1", added, removed)
	}
}

func TestDiffCoverageNotIncident(t *testing.T) {
	// A module disabled in current: a coverage change, never an incident.
	a := &Report{SSHDConfig: collectedSSHD(false)}
	b := &Report{SSHDConfig: Section[SSHDConfig]{Status: StatusDisabled}}
	d := Diff(a, b)
	if d.Changed() {
		t.Error("status transition must not be an incident")
	}
	if len(d.Coverage) != 1 || d.Coverage[0].Old != string(StatusCollected) || d.Coverage[0].New != string(StatusDisabled) {
		t.Errorf("expected one coverage change collected->disabled, got %+v", d.Coverage)
	}
}

func TestDiffFirstFieldAddedRemoved(t *testing.T) {
	// Adding a wildcard listener where there were none is an incident.
	a := collectedListeners()
	b := collectedListeners(ListeningSocket{Protocol: "tcp", BindClass: "wildcard", Port: 22, Program: "sshd"})
	d := Diff(&Report{ListeningTCP: a}, &Report{ListeningTCP: b})
	if len(d.Incidents) != 1 || d.Incidents[0].Kind != ChangeAdded {
		t.Errorf("expected 1 added incident, got %+v", d.Incidents)
	}
}
