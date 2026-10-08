//go:build !sqlite && !postgres && !memorybackend

package appbuild_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
)

// AC12 (D11): on fs a schema with sync refs assembles, so read-only tooling
// works, but the long-running hosts refuse it; a non-sync ref is fine.
func TestRequireSyncBackend_FS(t *testing.T) {
	const schema = `version: "1.0"
entities:
  ticket:
    label: Ticket
    plural: tickets
    id_prefix: "T-"
    id_type: sequential
    properties:
      basecamp: {type: external_ref, system: basecamp, sync: true}
`
	root := t.TempDir()
	writeMetamodelBody(t, root, schema)
	svc, err := discover(t, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	require.ErrorContains(t, appbuild.RequireSyncBackend(svc), "ticket.basecamp")

	root = t.TempDir()
	writeMetamodelBody(t, root, strings.Replace(schema, ", sync: true", "", 1))
	svc2, err := discover(t, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc2.Close() })
	require.NoError(t, appbuild.RequireSyncBackend(svc2))
}
