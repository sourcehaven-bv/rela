//go:build !postgres && !memorybackend && !sqlite

// fsstore-only for the reason fieldredaction_test.go gives: the fixture is
// markdown written to disk.

package appbuild_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

const notifyMetamodel = `version: "1.0"
piles:
  inbox:
    name: Notifications
    icon: bell
    read_state: true
    reasons:
      done: completed a task
entities:
  project:
    label: Project
    plural: projects
    id_type: manual
    properties:
      owner:
        type: string
  task:
    label: Task
    plural: tasks
    id_type: manual
    properties:
      status:
        type: string
  team:
    label: Team
    plural: teams
    id_type: manual
    properties:
      owner:
        type: string
  person:
    label: Person
    plural: people
    id_type: manual
    properties:
      email:
        type: string
        unique: true
relations:
  part-of:
    from: [task]
    to: [project, team]
automations:
  - name: tell-owner
    on:
      entity: [task]
      property: status
      becomes: done
    do:
      - notify: {pile: inbox, reason: done, to: {via: part-of, type: project, property: owner}}
`

// The declarative common case: a task of a project is completed by someone
// else, and the project owner gets a notification. The owner completing
// their own task gets none.
func TestNotify_ProjectOwnerLearnsOfCompletedTask(t *testing.T) {
	root := t.TempDir()
	writeMetamodelBody(t, root, notifyMetamodel)
	writePolicy(t, root, pileLifecyclePolicy+`  P-1: admin
  P-2: admin
  P-3: admin
`)
	writeFile(t, filepath.Join(root, "entities", "projects", "PRJ-1.md"),
		"---\nid: PRJ-1\ntype: project\nowner: P-1\n---\n")
	for _, id := range []string{"T-1", "T-2"} {
		writeFile(t, filepath.Join(root, "entities", "tasks", id+".md"),
			"---\nid: "+id+"\ntype: task\nstatus: open\n---\n")
		writeFile(t, filepath.Join(root, "relations", id+"--part-of--PRJ-1.md"),
			"---\nfrom: "+id+"\nrelation: part-of\nto: PRJ-1\n---\n")
	}
	// T-1 is also part of a team; its owner is no project owner and hears nothing.
	writeFile(t, filepath.Join(root, "entities", "teams", "TEAM-1.md"),
		"---\nid: TEAM-1\ntype: team\nowner: P-3\n---\n")
	writeFile(t, filepath.Join(root, "relations", "T-1--part-of--TEAM-1.md"),
		"---\nfrom: T-1\nrelation: part-of\nto: TEAM-1\n---\n")
	writeFile(t, filepath.Join(root, "entities", "people", "P-3.md"),
		"---\nid: P-3\ntype: person\nemail: lead@example.com\n---\n")
	writeFile(t, filepath.Join(root, "entities", "people", "P-1.md"),
		"---\nid: P-1\ntype: person\nemail: owner@example.com\n---\n")
	writeFile(t, filepath.Join(root, "entities", "people", "P-2.md"),
		"---\nid: P-2\ntype: person\nemail: worker@example.com\n---\n")

	svc, err := appbuildOnDisk(t, root)
	require.NoError(t, err)
	defer svc.Close()

	owner := boundAs(t, svc, principal.Principal{User: "P-1", RawUser: "owner@example.com", Tool: principal.ToolCLI})
	worker := boundAs(t, svc, principal.Principal{User: "P-2", RawUser: "worker@example.com", Tool: principal.ToolCLI})

	done := entity.Patch{Properties: map[string]any{"status": "done"}}
	_, err = svc.EntityManager().PatchEntity(worker, "T-1", done)
	require.NoError(t, err)
	_, err = svc.EntityManager().PatchEntity(owner, "T-2", done)
	require.NoError(t, err)

	inbox, err := svc.Piles().ByKey(owner, "inbox")
	require.NoError(t, err)
	require.Equal(t, []entity.Ref{{ID: "T-1"}}, inbox.Refs(), "only the task someone else completed")
	require.Equal(t, "P-2", inbox.Items[0].Actor)
	require.Equal(t, "done", inbox.Items[0].Reason)
	require.False(t, inbox.Items[0].Read())

	mine, err := svc.Piles().ByKey(worker, "inbox")
	require.NoError(t, err)
	require.Empty(t, mine.Items)

	lead := boundAs(t, svc, principal.Principal{User: "P-3", RawUser: "lead@example.com", Tool: principal.ToolCLI})
	theirs, err := svc.Piles().ByKey(lead, "inbox")
	require.NoError(t, err)
	require.Empty(t, theirs.Items, "to.type: project keeps the team owner out")
}
