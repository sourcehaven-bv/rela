package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/frontendroutes"
)

func TestWriteRoutesTable_containsAllRoutes(t *testing.T) {
	var buf bytes.Buffer
	if err := writeRoutesTable(&buf, frontendroutes.All()); err != nil {
		t.Fatalf("writeRoutesTable: %v", err)
	}
	out := buf.String()

	// Header row.
	if !strings.Contains(out, "NAME") || !strings.Contains(out, "PATH") ||
		!strings.Contains(out, "PARAMS") || !strings.Contains(out, "RETURN_TO") ||
		!strings.Contains(out, "NOTES") {

		t.Errorf("table header missing expected columns:\n%s", out)
	}

	// Every route must appear by name and path.
	for _, r := range frontendroutes.All() {
		if !strings.Contains(out, r.Name) {
			t.Errorf("route %q missing from output", r.Name)
		}
		if !strings.Contains(out, r.Path) {
			t.Errorf("path %q (route %q) missing from output", r.Path, r.Name)
		}
	}

	// form-edit should show return_to yes + the lua param names
	// (form_id, entity_id) so document authors learn the keys they pass
	// to rela.url.
	if !strings.Contains(out, "form_id") || !strings.Contains(out, "entity_id") {
		t.Errorf("expected Lua param names in table, got:\n%s", out)
	}
	if !strings.Contains(out, "yes") {
		t.Errorf("expected at least one route with return_to yes, got:\n%s", out)
	}
}

func TestWriteRoutesJSON_roundtrips(t *testing.T) {
	var buf bytes.Buffer
	if err := writeRoutesJSON(&buf, frontendroutes.All()); err != nil {
		t.Fatalf("writeRoutesJSON: %v", err)
	}

	var decoded []frontendroutes.Route
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("round-trip JSON decode: %v\n%s", err, buf.String())
	}
	want := frontendroutes.All()
	if len(decoded) != len(want) {
		t.Fatalf("route count mismatch: got %d, want %d", len(decoded), len(want))
	}
	for i := range want {
		if decoded[i].Name != want[i].Name || decoded[i].Path != want[i].Path {
			t.Errorf("route %d: got %+v, want %+v", i, decoded[i], want[i])
		}
	}
}

func TestRunRoutesCmd_invalidFormat(t *testing.T) {
	code := runRoutesCmd([]string{"--format", "yaml"})
	if code != 1 {
		t.Errorf("exit code = %d, want 1 for invalid format", code)
	}
}

func TestRunRoutesCmd_defaultTable(t *testing.T) {
	// runRoutesCmd writes to os.Stdout; we can't easily capture that without
	// plumbing io.Writer through. We still want to assert the happy-path
	// exit code. Output shape is covered by the writeRoutes* tests above.
	code := runRoutesCmd(nil)
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	code = runRoutesCmd([]string{"--format", "json"})
	if code != 0 {
		t.Errorf("json exit code = %d, want 0", code)
	}
}
