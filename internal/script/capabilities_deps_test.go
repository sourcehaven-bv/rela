package script

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
)

// nopMutator satisfies lua.Mutator so a writer runtime can be built. These
// tests assert on CAPABILITIES, never on mutation, so every method returns a
// sentinel error: if a probe script ever calls one, the test fails loudly
// instead of silently succeeding against a nil result.
type nopMutator struct{}

var errNoMutate = errors.New("capability test: mutation is not expected here")

func (nopMutator) CreateEntity(context.Context, *entity.Entity, entity.CreateOptions) (*entity.CreateResult, error) {
	return nil, errNoMutate
}
func (nopMutator) UpdateEntity(context.Context, *entity.Entity) (*entity.UpdateResult, error) {
	return nil, errNoMutate
}
func (nopMutator) PatchEntity(context.Context, string, entity.Patch) (*entity.UpdateResult, error) {
	return nil, errNoMutate
}
func (nopMutator) DeleteEntity(context.Context, string, bool) (*entity.DeleteResult, error) {
	return nil, errNoMutate
}
func (nopMutator) CreateRelation(context.Context, string, string, string, entity.RelationOptions) (*entity.Relation, error) {
	return nil, errNoMutate
}
func (nopMutator) DeleteRelation(context.Context, string, string, string) error { return errNoMutate }

// capProject writes a throwaway project whose script asserts, from inside Lua,
// that the capabilities it was promised are actually present. Asserting in Lua
// rather than on captured output is deliberate: Engine.execute owns the stdout
// buffer, so the script's own error is the only observable signal.
func capProject(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".rela"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".rela", "secrets.yaml"),
		[]byte("slack: SLACK-TOKEN\ndb_dsn: SECRET-DSN\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "scripts", "probe.lua"),
		[]byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

// TestDepsCapabilitiesSurviveExecuteFile is a REGRESSION test for a real defect
// found in review (TKT-YH52OM).
//
// Engine.execute passes lua.WithCapabilities UNCONDITIONALLY — plain
// ExecuteFile supplies an empty grant. While WithCapabilities assigned
// straight into r.caps, that empty option overwrote whatever
// ReadDeps.Capabilities had carried, so a deps-carried grant silently vanished.
//
// This is exactly the scheduler's call shape: it sets deps.Capabilities from
// the task's `capabilities:` block and then calls ExecuteFile, so EVERY
// scheduled task's declaration was a no-op. The fix makes an empty
// WithCapabilities a no-op rather than a revocation.
func TestDepsCapabilitiesSurviveExecuteFile(t *testing.T) {
	t.Parallel()
	root := capProject(t, `
if type(http) ~= "table" then error("http capability was dropped") end
if rela.secrets.slack ~= "SLACK-TOKEN" then error("granted secret was dropped") end
if rela.secrets.db_dsn ~= nil then error("UNGRANTED secret leaked") end
`)
	deps := lua.WriteDeps{
		ReadDeps: lua.ReadDeps{
			ProjectRoot:  root,
			Capabilities: lua.Capabilities{HTTP: true, Secrets: []string{"slack"}},
		},
		EntityManager: nopMutator{},
	}
	if err := NewEngine().ExecuteFile(context.Background(), "probe.lua", deps, nil, nil); err != nil {
		t.Fatalf("deps-carried capability grant did not reach the script: %v", err)
	}
}

// TestExecuteFileGrantsNothingByDefault is the other half: with no grant from
// either source, the script must reach nothing. Without this, the fix above
// could be "satisfied" by simply always granting.
func TestExecuteFileGrantsNothingByDefault(t *testing.T) {
	t.Parallel()
	root := capProject(t, `
if http ~= nil then error("http present without a grant") end
if ai ~= nil then error("ai present without a grant") end
if rela.secrets.slack ~= nil then error("secret present without a grant") end
`)
	deps := lua.WriteDeps{
		ReadDeps:      lua.ReadDeps{ProjectRoot: root},
		EntityManager: nopMutator{},
	}
	if err := NewEngine().ExecuteFile(context.Background(), "probe.lua", deps, nil, nil); err != nil {
		t.Fatalf("ungranted runtime reached a capability: %v", err)
	}
}

// TestExplicitCapabilitiesStillOverrideDeps pins that the per-execution grant
// still wins when it actually says something — the automation path relies on
// it (ExecuteFileWithCapabilities carries the action's block).
func TestExplicitCapabilitiesStillOverrideDeps(t *testing.T) {
	t.Parallel()
	root := capProject(t, `
if type(ai) ~= "table" then error("explicit grant did not apply") end
if rela.secrets.db_dsn ~= "SECRET-DSN" then error("explicit secret did not apply") end
`)
	deps := lua.WriteDeps{
		// Deps say http+slack; the explicit call says ai+db_dsn and must win.
		ReadDeps: lua.ReadDeps{
			ProjectRoot:  root,
			Capabilities: lua.Capabilities{HTTP: true, Secrets: []string{"slack"}},
		},
		EntityManager: nopMutator{},
	}
	err := NewEngine().ExecuteFileWithCapabilities(context.Background(), "probe.lua", deps,
		nil, nil, lua.Capabilities{AI: true, Secrets: []string{"db_dsn"}})
	if err != nil {
		t.Fatalf("explicit grant did not take effect: %v", err)
	}
}
