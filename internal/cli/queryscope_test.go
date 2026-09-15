package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
)

// scopedTaakServices builds a project whose `taak` type declares a DEFAULT
// query scope hiding archived rows, seeded with one archived and one live
// task.
//
// `~=` does not lower to a store predicate, so the archived row can only
// disappear if something deliberately applies the scope in Go.
func scopedTaakServices(t *testing.T) *readServices {
	t.Helper()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"taak": {
				Label:    "Taak",
				Plural:   "taken",
				IDPrefix: "TAAK-",
				Properties: map[string]metamodel.PropertyDef{
					"title":  {Type: "string"},
					"status": {Type: "string"},
				},
				QueryScopes: map[string]string{
					"default": "entity.status ~= 'gearchiveerd'",
					"archief": "entity.status == 'gearchiveerd'",
				},
			},
		},
	}
	seeder := newStoreSeeder(meta)
	seeder.addEntity(testutil.EntityFor(meta, "taak").
		ID("TAAK-001").With("title", "Live task").With("status", "todo"))
	seeder.addEntity(testutil.EntityFor(meta, "taak").
		ID("TAAK-002").With("title", "Archived task").With("status", "gearchiveerd"))
	return seeder.build(t).read
}

// TestListCmd_IgnoresQueryScopes is AC6 for `rela list` (TKT-EVR2TU).
//
// The CLI is an operator's view of the store, not a rendered screen. An
// operator running `rela list taak` to check what is there must not be shown a
// subset chosen by a presentation declaration — particularly since the CLI is
// the tool they would reach for to investigate why a screen looks wrong.
func TestListCmd_IgnoresQueryScopes(t *testing.T) {
	svc := scopedTaakServices(t)
	buf := withOutput(t, output.FormatJSON)

	if err := (&ListCmd{Type: "taak"}).Run(context.Background(), svc); err != nil {
		t.Fatalf("ListCmd.Run: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "TAAK-002") {
		t.Fatalf("`rela list` dropped the archived row, so a schema default query "+
			"scope reached the CLI. Output:\n%s", got)
	}
	if !strings.Contains(got, "TAAK-001") {
		t.Fatalf("control row missing — the fixture, not the scope, is wrong:\n%s", got)
	}
}

// TestExportCmd_IgnoresQueryScopes is AC6 for `rela export`.
//
// Export is the surface where a silent narrowing is worst: the output is
// archived, handed to another system, or used as a backup, and nothing
// downstream can tell that rows were withheld. A scoped export would produce a
// file that looks complete and is not.
//
// Note this is the CLI export, which is deliberately NOT the data-entry view
// export. That one IS pinned to its view's scope, on the user's decision —
// because it exports what the screen shows, by definition.
func TestExportCmd_IgnoresQueryScopes(t *testing.T) {
	tests := []struct {
		name string
		cmd  ExportCmd
	}{
		{name: "by type", cmd: ExportCmd{Type: "taak", Format: "json"}},
		{name: "all", cmd: ExportCmd{All: true, Format: "json"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := scopedTaakServices(t)
			got, err := captureStdout(t, func() error {
				return tc.cmd.Run(context.Background(), svc)
			})
			if err != nil {
				t.Fatalf("ExportCmd.Run: %v", err)
			}
			if !strings.Contains(got, "TAAK-002") {
				t.Fatalf("export omitted the archived row; an export that silently "+
					"withholds rows produces a file nothing downstream can audit. "+
					"Output:\n%s", got)
			}
			if !strings.Contains(got, "TAAK-001") {
				t.Fatalf("control row missing — the fixture, not the scope, is wrong:\n%s", got)
			}
		})
	}
}
