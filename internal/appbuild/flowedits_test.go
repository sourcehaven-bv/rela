package appbuild_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/flowview"
)

// outsideEditsBlock withholds documents and stage while onboarding is in
// collect, and keeps name to holders of onboarding.start.
const outsideEditsBlock = `    outside_edits:
      states:
        collect:
          properties:
            documents: deny
            stage: deny
            name: { permission: onboarding.start }
`

// holdFlow is a second flow on person that withholds documents while held.
const holdFlow = `  hold:
    label: Hold
    subject: person
    start:
      - on: manual
        permission: onboarding.start
    initial: held
    outside_edits:
      states:
        held:
          properties:
            documents: deny
    states:
      held:
        waiting_for: [hr]
        writes: [salary]
        on:
          change:
            to: released
            when: "entity.documents == false"
      released:
        final: true
`

// outsideEditsProject is the onboarding sample with outsideEditsBlock and
// holdFlow, and dave as a clerk: he may update a person but holds no
// permission.
func outsideEditsProject(t *testing.T) (*appbuild.Services, *appbuild.FlowControl) {
	t.Helper()
	return outsideEditsProjectIn(t, writeFlowProject(t))
}

// outsideEditsProjectIn adds the rules of [outsideEditsProject] to the flow
// project at root. Each of denied is one more property onboarding's collect
// state denies.
func outsideEditsProjectIn(t *testing.T, root string, denied ...string) (*appbuild.Services, *appbuild.FlowControl) {
	t.Helper()
	flows, err := os.ReadFile(filepath.Join(root, "flows.yaml"))
	require.NoError(t, err)
	var block strings.Builder
	block.WriteString(outsideEditsBlock)
	for _, p := range denied {
		block.WriteString("            " + p + ": deny\n")
	}
	f := strings.Replace(string(flows), "    initial: collect\n", "    initial: collect\n"+block.String(), 1)
	require.NoError(t, os.WriteFile(filepath.Join(root, "flows.yaml"), []byte(f+holdFlow), 0o600))
	policy, err := os.ReadFile(filepath.Join(root, "acl.yaml"))
	require.NoError(t, err)
	p := strings.Replace(string(policy), "assignments:\n", "assignments:\n  alice: hr\n  dave: clerk\n", 1)
	p = strings.Replace(p, "roles:\n", "roles:\n  clerk:\n    read: [person]\n    update: [person]\n", 1)
	writePolicy(t, root, p)
	svc, err := appbuildOnDisk(t, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	require.NotNil(t, appbuild.FlowEdits(svc))
	ctl := appbuild.FlowControlFor(svc)
	require.NotNil(t, ctl)
	return svc, ctl
}

func requireFlowReadOnly(t *testing.T, err error, prop string) {
	t.Helper()
	var d *affordances.FieldWriteError
	require.ErrorAs(t, err, &d)
	require.Equal(t, "field-affordance:read-only:"+prop, d.RuleID())
}

// TestOutsideEdits_GatedWrites pins the ceiling on the field-gated manager
// while onboarding is in collect: deny refuses everyone, a permission rule
// refuses a caller without it, and an unruled property stays writable.
func TestOutsideEdits_GatedWrites(t *testing.T) {
	tests := []struct {
		name string
		user string
		prop string
		val  any
		deny bool
	}{
		{name: "deny refuses a clerk", user: "dave", prop: "documents", val: true, deny: true},
		{name: "deny refuses hr", user: "alice", prop: "documents", val: true, deny: true},
		{name: "permission rule refuses a non-holder", user: "dave", prop: "name", val: "Bea", deny: true},
		{name: "permission rule admits a holder", user: "alice", prop: "name", val: "Bea"},
		{name: "unruled property", user: "dave", prop: "salary", val: 10},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, ctl := outsideEditsProject(t)
			addFlowPerson(t, svc, "PER-1", map[string]any{"name": "Ada"})
			_, err := ctl.Start(flowUser(t, svc, "alice"), "onboarding", "PER-1")
			require.NoError(t, err)
			mgr := entitymanager.FieldGated(svc.EntityManager())
			_, err = mgr.PatchEntity(flowUser(t, svc, tc.user), "PER-1",
				entity.Patch{Properties: map[string]any{tc.prop: tc.val}})
			if tc.deny {
				requireFlowReadOnly(t, err, tc.prop)
				return
			}
			require.NoError(t, err)
		})
	}
}

// TestOutsideEdits_RunOwnWrites pins the exemption: the run's own output
// and step write a property its outside_edits deny, and the ceiling lifts
// once the run has finished.
func TestOutsideEdits_RunOwnWrites(t *testing.T) {
	svc, ctl := outsideEditsProject(t)
	addFlowPerson(t, svc, "PER-1", map[string]any{"name": "Ada"})
	alice := flowUser(t, svc, "alice")
	id, err := ctl.Start(alice, "onboarding", "PER-1")
	require.NoError(t, err)
	subj, err := svc.Store().GetEntity(context.Background(), entity.Ref{ID: "PER-1"})
	require.NoError(t, err)
	require.Equal(t, "collecting", subj.Properties["stage"], "the run's own output writes a denied property")

	run, err := ctl.Run(alice, id)
	require.NoError(t, err)
	got, err := ctl.Complete(alice, id, run.Generation, map[string]any{"documents": true})
	require.NoError(t, err, "the step's own write is not narrowed by its run")
	require.True(t, got.Finished)

	mgr := entitymanager.FieldGated(svc.EntityManager())
	_, err = mgr.PatchEntity(flowUser(t, svc, "dave"), "PER-1",
		entity.Patch{Properties: map[string]any{"documents": false, "stage": "done"}})
	require.NoError(t, err, "a finished run withholds nothing")
}

// TestOutsideEdits_OtherRunStillNarrows: a step write is exempt from its
// own run only; a second flow's run on the subject still refuses it.
func TestOutsideEdits_OtherRunStillNarrows(t *testing.T) {
	svc, ctl := outsideEditsProject(t)
	addFlowPerson(t, svc, "PER-1", map[string]any{"name": "Ada"})
	alice := flowUser(t, svc, "alice")
	_, err := ctl.Start(alice, "hold", "PER-1")
	require.NoError(t, err)
	id, err := ctl.Start(alice, "onboarding", "PER-1")
	require.NoError(t, err)
	run, err := ctl.Run(alice, id)
	require.NoError(t, err)
	_, err = ctl.Complete(alice, id, run.Generation, map[string]any{"documents": true})
	requireFlowReadOnly(t, err, "documents")
	require.ErrorIs(t, err, flowview.ErrForbidden, "the transports answer a refused step with 403")
}

// TestOutsideEdits_RenameRefusedWhileHeld: a subject an unfinished run
// holds keeps its id, so the rules cannot be lifted by a rename. A dry run
// says so too. Once the run has ended, the rename goes through.
func TestOutsideEdits_RenameRefusedWhileHeld(t *testing.T) {
	root := writeFlowProject(t)
	schema, err := os.ReadFile(filepath.Join(root, "metamodel.yaml"))
	require.NoError(t, err)
	manual := strings.Replace(string(schema), "id_prefix: \"PER-\"\n    id_type: sequential",
		"id_prefix: \"PER-\"\n    id_type: manual", 1)
	require.NotEqual(t, string(schema), manual, "rename needs manual ids")
	writeMetamodelBody(t, root, manual)
	svc, ctl := outsideEditsProjectIn(t, root)
	addFlowPerson(t, svc, "PER-1", map[string]any{"name": "Ada"})
	alice := flowUser(t, svc, "alice")
	id, err := ctl.Start(alice, "onboarding", "PER-1")
	require.NoError(t, err)
	for _, dry := range []bool{true, false} {
		_, err = svc.EntityManager().RenameEntity(alice, "PER-1", "PER-9", entity.RenameOptions{DryRun: dry})
		require.ErrorIs(t, err, entitymanager.ErrRenameBlocked, "dry run %v", dry)
	}
	mgr := entitymanager.FieldGated(svc.EntityManager())
	_, err = mgr.PatchEntity(flowUser(t, svc, "dave"), "PER-1",
		entity.Patch{Properties: map[string]any{"documents": true}})
	requireFlowReadOnly(t, err, "documents")

	require.NoError(t, ctl.Cancel(alice, id))
	_, err = svc.EntityManager().RenameEntity(alice, "PER-1", "PER-9", entity.RenameOptions{})
	require.NoError(t, err)
}

// TestOutsideEdits_UngatedWritesUnchanged: the manager without the field
// gate (CLI, stdio MCP) is not narrowed.
func TestOutsideEdits_UngatedWritesUnchanged(t *testing.T) {
	svc, ctl := outsideEditsProject(t)
	addFlowPerson(t, svc, "PER-1", map[string]any{"name": "Ada"})
	_, err := ctl.Start(flowUser(t, svc, "alice"), "onboarding", "PER-1")
	require.NoError(t, err)
	_, err = svc.EntityManager().PatchEntity(flowUser(t, svc, "dave"), "PER-1",
		entity.Patch{Properties: map[string]any{"documents": true}})
	require.NoError(t, err)
}
