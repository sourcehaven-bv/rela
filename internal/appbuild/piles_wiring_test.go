//go:build !postgres && !memorybackend && !sqlite

// fsstore-only for the reason fieldredaction_test.go gives: the fixture is
// markdown written to disk.

package appbuild_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

const pileAutomationMetamodel = `version: "1.0"
entities:
  ticket:
    label: Ticket
    plural: tickets
    id_prefix: "TKT-"
    id_type: sequential
    properties:
      title:
        type: string
relations: {}
automations:
  - name: to-inbox
    on:
      entity: [ticket]
      property: title
    do:
      - add_to_pile: {pile: Inbox}
  - name: scripted
    on:
      entity: [ticket]
      property: title
    do:
      - lua: |
          rela.piles.add{pile = "Scripted", entities = {"TKT-1"}}
`

// The assembled cascade reaches the piles service on both of its paths: the
// add_to_pile action and rela.piles.add in a Lua action. Each lands on the
// acting user's pile.
func TestPiles_CascadeWiring(t *testing.T) {
	root := t.TempDir()
	writeMetamodelBody(t, root, pileAutomationMetamodel)
	tickets := filepath.Join(root, "entities", "tickets")
	require.NoError(t, os.MkdirAll(tickets, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tickets, "TKT-1.md"),
		[]byte("---\nid: TKT-1\ntype: ticket\ntitle: Old\n---\n"), 0o600))

	svc, err := appbuildOnDisk(t, root)
	require.NoError(t, err)
	defer svc.Close()

	ctx := bobCtx(principal.ToolDataEntry)
	res, err := svc.EntityManager().PatchEntity(ctx, "TKT-1", entity.Patch{
		Properties: map[string]any{"title": "New"},
	})
	require.NoError(t, err)
	require.Empty(t, res.AutomationErrors)

	for _, name := range []string{"Inbox", "Scripted"} {
		p, err := svc.Piles().ByName(ctx, name)
		require.NoError(t, err, "pile %q must exist after the cascade", name)
		require.Equal(t, []entity.Ref{{ID: "TKT-1"}}, p.Refs(), "pile %q", name)
	}
}
