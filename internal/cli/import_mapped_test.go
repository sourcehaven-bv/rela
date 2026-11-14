package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

const mappedImportMetamodel = `version: "1.0"
entities:
  control:
    label: Control
    plural: controls
    id_prefix: "CTL-"
    id_type: sequential
    properties:
      code: {type: string, unique: true}
      title: {type: string}
`

const mappedImportMapping = `entities:
  controls:
    type: control
    select: $.controls
    key: code
    properties:
      code: id
      title: title
`

type mappedImportOutput struct {
	Plan struct {
		Creates []struct {
			Key string `json:"key"`
		} `json:"creates"`
		Updates   []json.RawMessage `json:"updates"`
		Unchanged int               `json:"unchanged"`
		Errors    []json.RawMessage `json:"errors"`
	} `json:"plan"`
	Result *struct {
		Created int `json:"created"`
	} `json:"result"`
}

// TestImportMapped_CLI runs `rela import --mapping` on a filesystem
// project: a dry run writes nothing, a run writes, and a second run plans
// nothing.
func TestImportMapped_CLI(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".rela"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "imports"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "metamodel.yaml"), []byte(mappedImportMetamodel), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "imports", "catalog.yaml"), []byte(mappedImportMapping), 0o600))
	input := filepath.Join(root, "catalog.json")
	require.NoError(t, os.WriteFile(input, []byte(`{"controls": [{"id": "a", "title": "A"}, {"id": "b"}]}`), 0o600))

	svc, err := appbuild.Discover(root, script.NewEngine())
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	prev := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = prev })

	run := func(args ...string) mappedImportOutput {
		t.Helper()
		got, rerr := runCLI(t, svc, "", append([]string{"-o", "json", "import", input, "--mapping", "catalog"}, args...)...)
		require.NoError(t, rerr, got)
		var o mappedImportOutput
		require.NoError(t, json.Unmarshal([]byte(got), &o), got)
		return o
	}
	count := func() int {
		n := 0
		for _, err := range svc.Store().ListEntities(leaseCtx(), store.EntityQuery{Type: "control", Faces: store.AllFaces()}) {
			require.NoError(t, err)
			n++
		}
		return n
	}

	dry := run("--dry-run")
	require.Len(t, dry.Plan.Creates, 2)
	require.Nil(t, dry.Result)
	require.Equal(t, 0, count())

	applied := run()
	require.NotNil(t, applied.Result)
	require.Equal(t, 2, applied.Result.Created)
	require.Equal(t, 2, count())

	again := run()
	require.Empty(t, again.Plan.Creates)
	require.Empty(t, again.Plan.Updates)
	require.Equal(t, 2, again.Plan.Unchanged)

	_, err = runCLI(t, svc, "", "import", input, "--mapping", "catalog", "--update")
	require.ErrorContains(t, err, "do not apply with --mapping")
	_, err = runCLI(t, svc, "", "import", input, "--mapping", "Nope")
	require.ErrorContains(t, err, "invalid import name")
	_, err = runCLI(t, svc, "", "import", input, "--mapping", "absent")
	require.ErrorContains(t, err, "imports/absent.yaml")
}
