//go:build !postgres && !memorybackend && !sqlite

// fsstore-only for the reason fieldredaction_test.go gives: the fixture is
// markdown written to disk.

package appbuild_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// GH-2118: a record of stored IdP roles does not outlive the person it is
// bound to. Deleting or renaming the person through the entity manager
// drops it, and Forget erases it on an operator's request.
func TestIDPRoles_ErasedWithThePerson(t *testing.T) {
	root := t.TempDir()
	writeMetamodelBody(t, root, `version: "1.0"
entities:
  person:
    label: Person
    plural: persons
    id_type: manual
    properties:
      email:
        type: string
        unique: true
relations: {}
`)
	writePolicy(t, root, `user_entity_type: person
principal_property: email
roles:
  admin:
    read: ["*"]
    create: ["*"]
    update: ["*"]
    delete: ["*"]
    rename: ["*"]
  cpo:
    read: ["*"]
asserted_role_assignments:
  idp-cpo: cpo
assignments:
  system:provisioner: admin
`)
	for id, email := range map[string]string{"PER-1": "a@x", "PER-2": "b@x", "PER-3": "c@x"} {
		writeFile(t, filepath.Join(root, "entities", "persons", id+".md"),
			"---\nid: "+id+"\ntype: person\nemail: "+email+"\n---\n")
	}
	svc, err := appbuildOnDisk(t, root)
	require.NoError(t, err)
	defer svc.Close()

	ctx := context.Background()
	roles := appbuild.IDPRoles(svc)
	require.NotNil(t, roles)
	for _, subject := range []string{"a@x", "b@x", "c@x"} {
		require.NoError(t, roles.Set(ctx, subject, "org_1", []string{"idp-cpo"}))
	}
	admin := bindAt(principal.With(ctx,
		principal.Principal{User: principal.UserProvisioner, Tool: principal.ToolScheduler}), t, svc)
	_, err = svc.EntityManager().DeleteEntity(admin, "PER-1", false)
	require.NoError(t, err)
	_, err = svc.EntityManager().RenameEntity(admin, "PER-2", "PER-20", entity.RenameOptions{})
	require.NoError(t, err)

	// Forget reports what was still stored: nothing for the deleted and the
	// renamed person, one record for the person still present.
	for id, want := range map[string]int{"PER-1": 0, "PER-2": 0, "PER-3": 1} {
		got, ferr := roles.Forget(ctx, id)
		require.NoError(t, ferr)
		require.Equal(t, want, got, id)
	}
	n, err := roles.Forget(ctx, "PER-3")
	require.NoError(t, err)
	require.Zero(t, n, "erased")
}
