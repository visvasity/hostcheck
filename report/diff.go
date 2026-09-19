// Copyright (c) 2026 Visvasity LLC

package report

import (
	"encoding/json"
	"slices"
)

// ChangeKind classifies a single difference.
type ChangeKind string

const (
	ChangeAdded    ChangeKind = "added"
	ChangeRemoved  ChangeKind = "removed"
	ChangeModified ChangeKind = "modified"
)

// Change is one difference between a baseline and a current report, identified by
// a dotted path (e.g. "listening_tcp.sockets" or "sshd_config.password_authentication").
// Old and New hold compact JSON of the affected values (empty where not applicable).
type Change struct {
	Path string     `json:"path"`
	Kind ChangeKind `json:"kind"`
	Old  string     `json:"old,omitempty"`
	New  string     `json:"new,omitempty"`
}

// DiffResult separates genuine host-state changes (Incidents) from status and
// coverage transitions (Coverage). Only Incidents drive change detection: a
// module toggled off, or a collector added/removed across an agent upgrade, is a
// coverage change, not an incident.
type DiffResult struct {
	Incidents []Change `json:"incidents,omitempty"`
	Coverage  []Change `json:"coverage,omitempty"`
}

// Changed reports whether any incident-level change was detected.
func (d DiffResult) Changed() bool { return len(d.Incidents) > 0 }

// envelopeKeys are the top-level report fields that are not collection sections
// and are excluded from diffing (generated_at changes every run; the rest are
// build/config metadata, not host state).
var envelopeKeys = map[string]bool{
	"schema_version": true, "generated_at": true, "mode": true,
	"platform": true, "enabled_modules": true,
}

// Diff compares two reports section by section. For a module collected in both,
// it diffs the section data field by field (added/removed/modified) into
// Incidents. For any other transition (a section's status changed, or it began
// or stopped being collected), it records a Coverage change instead.
func Diff(old, new *Report) DiffResult {
	om := reportSections(old)
	nm := reportSections(new)

	var res DiffResult
	for _, key := range sortedUnion(om, nm) {
		os := parseSection(om[key])
		ns := parseSection(nm[key])
		switch {
		case os.collected() && ns.collected():
			res.Incidents = append(res.Incidents, diffValues(key, decodeJSON(os.Data), decodeJSON(ns.Data))...)
		case os.Status != ns.Status:
			res.Coverage = append(res.Coverage, Change{Path: key, Kind: ChangeModified, Old: os.Status, New: ns.Status})
		}
	}
	return res
}

// reportSections marshals a report and returns its collection sections keyed by
// JSON field name, excluding the envelope.
func reportSections(r *Report) map[string]json.RawMessage {
	m := make(map[string]json.RawMessage)
	if r == nil {
		return m
	}
	data, err := json.Marshal(r)
	if err != nil {
		return m
	}
	_ = json.Unmarshal(data, &m)
	for k := range envelopeKeys {
		delete(m, k)
	}
	return m
}

type rawSection struct {
	Status string          `json:"status"`
	Data   json.RawMessage `json:"data"`
}

func parseSection(raw json.RawMessage) rawSection {
	var s rawSection
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &s)
	}
	return s
}

func (s rawSection) collected() bool {
	return s.Status == string(StatusCollected) && len(s.Data) > 0
}

func decodeJSON(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	var v any
	_ = json.Unmarshal(raw, &v)
	return v
}

func compact(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

// diffValues recursively compares two decoded JSON values, emitting a Change per
// leaf difference. Objects recurse by key; arrays are compared as sets (correct
// because collector output is canonically sorted and deduplicated); scalars and
// type mismatches compare by value.
func diffValues(path string, a, b any) []Change {
	// Normalize JSON null against a typed sibling (a nil slice marshals to null),
	// so an empty->populated array yields element-level added/removed rather than
	// a whole-value "modified null -> [...]".
	if a == nil && b == nil {
		return nil
	}
	if a == nil {
		switch b.(type) {
		case []any:
			a = []any{}
		case map[string]any:
			a = map[string]any{}
		}
	} else if b == nil {
		switch a.(type) {
		case []any:
			b = []any{}
		case map[string]any:
			b = map[string]any{}
		}
	}

	switch av := a.(type) {
	case map[string]any:
		bv, ok := b.(map[string]any)
		if !ok {
			return []Change{{Path: path, Kind: ChangeModified, Old: compact(a), New: compact(b)}}
		}
		var out []Change
		for _, k := range sortedKeys(av, bv) {
			ax, aok := av[k]
			bx, bok := bv[k]
			cp := path + "." + k
			switch {
			case aok && !bok:
				out = append(out, Change{Path: cp, Kind: ChangeRemoved, Old: compact(ax)})
			case !aok && bok:
				out = append(out, Change{Path: cp, Kind: ChangeAdded, New: compact(bx)})
			default:
				out = append(out, diffValues(cp, ax, bx)...)
			}
		}
		return out
	case []any:
		bv, ok := b.([]any)
		if !ok {
			return []Change{{Path: path, Kind: ChangeModified, Old: compact(a), New: compact(b)}}
		}
		return diffArrays(path, av, bv)
	default:
		if compact(a) != compact(b) {
			return []Change{{Path: path, Kind: ChangeModified, Old: compact(a), New: compact(b)}}
		}
		return nil
	}
}

// diffArrays compares two arrays as sets of compact-JSON elements.
func diffArrays(path string, a, b []any) []Change {
	inA := make(map[string]bool, len(a))
	for _, e := range a {
		inA[compact(e)] = true
	}
	inB := make(map[string]bool, len(b))
	for _, e := range b {
		inB[compact(e)] = true
	}
	var out []Change
	for _, e := range a {
		if s := compact(e); !inB[s] {
			out = append(out, Change{Path: path, Kind: ChangeRemoved, Old: s})
		}
	}
	for _, e := range b {
		if s := compact(e); !inA[s] {
			out = append(out, Change{Path: path, Kind: ChangeAdded, New: s})
		}
	}
	return out
}

func sortedUnion(a, b map[string]json.RawMessage) []string {
	seen := make(map[string]bool, len(a)+len(b))
	var keys []string
	for k := range a {
		if !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	for k := range b {
		if !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	return keys
}

func sortedKeys(a, b map[string]any) []string {
	seen := make(map[string]bool, len(a)+len(b))
	var keys []string
	for k := range a {
		if !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	for k := range b {
		if !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	return keys
}
