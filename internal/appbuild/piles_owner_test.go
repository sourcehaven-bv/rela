//go:build !postgres && !memorybackend && !sqlite

// fsstore-only for the reason fieldredaction_test.go gives: the fixture is
// markdown written to disk.

package appbuild_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/piles"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// The scheduler may read tickets only; the provisioner reads everything.
const pileOwnerPolicy = `user_entity_type: person
principal_property: email
roles:
  viewer:
    read: [ticket]
  admin:
    read: ["*"]
assignments:
  system:scheduler: viewer
  system:provisioner: admin
`

func systemCtx(user string) context.Context {
	return principal.With(context.Background(), principal.Principal{User: user, Tool: principal.ToolScheduler})
}

// A push to another user checks the owner through the PUSHER's read gate, so
// "unknown owner" is the same answer for a missing person and a hidden one.
func TestPiles_PushOwnerCheckUsesCallerGate(t *testing.T) {
	root := t.TempDir()
	writeMetamodelBody(t, root, `version: "1.0"
entities:
  ticket:
    label: Ticket
    plural: tickets
    id_type: manual
    properties:
      title:
        type: string
  person:
    label: Person
    plural: people
    id_type: manual
    properties:
      email:
        type: string
        unique: true
relations: {}
`)
	writePolicy(t, root, pileOwnerPolicy)
	writeFile(t, filepath.Join(root, "entities", "tickets", "T-1.md"), "---\nid: T-1\ntype: ticket\n---\n")
	writeFile(t, filepath.Join(root, "entities", "people", "P-1.md"),
		"---\nid: P-1\ntype: person\nemail: p1@example.com\n---\n")

	svc, err := appbuildOnDisk(t, root)
	require.NoError(t, err)
	defer svc.Close()
	push := piles.PushRequest{Owner: "P-1", Pile: "Inbox", Create: true, Refs: []entity.Ref{{ID: "T-1"}}}

	_, err = svc.Piles().Push(systemCtx(principal.UserScheduler), push)
	require.ErrorIs(t, err, piles.ErrUnknownOwner, "the scheduler cannot read P-1, so to it P-1 does not exist")

	missing := push
	missing.Owner = "P-404"
	_, err = svc.Piles().Push(systemCtx(principal.UserProvisioner), missing)
	require.ErrorIs(t, err, piles.ErrUnknownOwner)

	_, err = svc.Piles().Push(systemCtx(principal.UserProvisioner), push)
	require.NoError(t, err, "a pusher that can read P-1 may push to P-1")

	owner := principal.With(context.Background(),
		principal.Principal{User: "P-1", RawUser: "p1@example.com", Tool: principal.ToolCLI})
	got, err := svc.Piles().ByName(owner, "Inbox")
	require.NoError(t, err)
	require.Len(t, got.Items, 1)
}

const pileLifecyclePolicy = `user_entity_type: person
principal_property: email
roles:
  admin:
    read: ["*"]
    create: ["*"]
    update: ["*"]
    delete: ["*"]
assignments:
  system:provisioner: admin
`

// The full wired lifecycle under person mapping: an add_to_pile automation
// pushes a ticket onto its assignee's Inbox, renaming the ticket rewrites the
// item, renaming the person moves the pile, and deleting the ticket drops it.
func TestPiles_AssigneeLifecycle(t *testing.T) {
	root := t.TempDir()
	writeMetamodelBody(t, root, `version: "1.0"
entities:
  ticket:
    label: Ticket
    plural: tickets
    id_type: manual
    properties:
      title:
        type: string
      assignee:
        type: string
  person:
    label: Person
    plural: people
    id_type: manual
    properties:
      email:
        type: string
        unique: true
relations: {}
automations:
  - name: assignee-inbox
    on:
      entity: [ticket]
      property: assignee
    do:
      - add_to_pile: {pile: Inbox, owner: "{{new.assignee}}"}
`)
	writePolicy(t, root, pileLifecyclePolicy)
	writeFile(t, filepath.Join(root, "entities", "tickets", "T-1.md"), "---\nid: T-1\ntype: ticket\n---\n")
	writeFile(t, filepath.Join(root, "entities", "people", "P-1.md"),
		"---\nid: P-1\ntype: person\nemail: p1@example.com\n---\n")

	svc, err := appbuildOnDisk(t, root)
	require.NoError(t, err)
	defer svc.Close()
	admin := systemCtx(principal.UserProvisioner)
	ownerCtx := func(id string) context.Context {
		return principal.With(context.Background(),
			principal.Principal{User: id, RawUser: "p1@example.com", Tool: principal.ToolCLI})
	}
	inbox := func(owner string) []entity.Ref {
		t.Helper()
		p, gerr := svc.Piles().ByName(ownerCtx(owner), "Inbox")
		require.NoError(t, gerr)
		return p.Refs()
	}

	res, err := svc.EntityManager().PatchEntity(admin, "T-1", entity.Patch{
		Properties: map[string]any{"assignee": "P-1"},
	})
	require.NoError(t, err)
	require.Empty(t, res.AutomationErrors)
	require.Equal(t, []entity.Ref{{ID: "T-1"}}, inbox("P-1"), "the automation pushes to the assignee")

	_, err = svc.EntityManager().RenameEntity(admin, "T-1", "T-2", entity.RenameOptions{})
	require.NoError(t, err)
	require.Equal(t, []entity.Ref{{ID: "T-2"}}, inbox("P-1"), "an item follows its entity's rename")

	_, err = svc.EntityManager().RenameEntity(admin, "P-1", "P-9", entity.RenameOptions{})
	require.NoError(t, err)
	require.Equal(t, []entity.Ref{{ID: "T-2"}}, inbox("P-9"), "a pile follows its owner's rename")

	_, err = svc.EntityManager().DeleteEntity(admin, "T-2", false)
	require.NoError(t, err)
	require.Empty(t, inbox("P-9"), "deleting an entity drops its items")
}
