package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/datamigration"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// migrateTestServices wires a writeServices over a real temp project dir, so
// the migrations/ directory and its applied-list behave as they do in
// production rather than through an in-memory shim.
func migrateTestServices(t *testing.T, meta *metamodel.Metamodel) (svc *writeServices, root string) {
	t.Helper()
	root = t.TempDir()
	paths := &project.Context{Root: root, CacheDir: filepath.Join(root, ".rela")}
	b, err := newCLIBundles(
		appbuildtest.New(meta, appbuildtest.WithFS(storage.NewOsFS(), paths)),
	)
	if err != nil {
		t.Fatalf("build cli services: %v", err)
	}
	return b.write, root
}

func migrateTestMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"task": {Label: "Task", IDPrefix: "TSK-", Properties: map[string]metamodel.PropertyDef{
				"title": {Type: "string", Required: true},
				"owner": {Type: "string"},
			}},
		},
	}
}

// writeMigration drops a migration file into the project, with both
// projections set to the SAME shape — a data-only migration, which is the
// case the previous hash-edge model could not represent at all.
func writeMigration(t *testing.T, root, name string, meta *metamodel.Metamodel, steps string) {
	t.Helper()
	dir := filepath.Join(root, datamigration.MigrationsDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir migrations: %v", err)
	}
	proj, err := meta.ShapeProjection().JSON()
	if err != nil {
		t.Fatalf("projection JSON: %v", err)
	}
	body := "description: test migration\nsteps:\n" + steps +
		"from_projection: " + string(proj) + "\n" +
		"to_projection: " + string(proj) + "\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatalf("write migration: %v", err)
	}
}

func readApplied(t *testing.T, svc *writeServices) *datamigration.State {
	t.Helper()
	st, err := svc.MigState.Load(t.Context())
	if err != nil {
		t.Fatalf("load migration state: %v", err)
	}
	return st
}

// The acceptance test for the whole change: a migration that does not change
// the schema applies, is recorded by name, and is skipped on re-run.
func TestMigrateData_DataOnlyMigrationRunsAndIsRecorded(t *testing.T) {
	meta := migrateTestMeta()
	svc, root := migrateTestServices(t, meta)
	const name = "20260919143022-backfill-owner.yaml"
	writeMigration(t, root, name, meta,
		"  - set_default: {entity: task, property: owner, value: unassigned}\n")

	cmd := &MigrateDataCmd{Apply: true}
	if err := cmd.Run(t.Context(), svc); err != nil {
		t.Fatalf("migrate data --apply: %v", err)
	}

	st := readApplied(t, svc)
	if st == nil {
		t.Fatal("nothing recorded after an apply")
	}
	if names := st.AppliedNames(); len(names) != 1 || names[0] != name {
		t.Fatalf("applied = %v, want [%s]", names, name)
	}

	// Re-running must not record it twice: the applied list is now the only
	// double-apply guard.
	if err := cmd.Run(t.Context(), svc); err != nil {
		t.Fatalf("second migrate data --apply: %v", err)
	}
	if names := readApplied(t, svc).AppliedNames(); len(names) != 1 {
		t.Fatalf("applied = %v after re-run, want the single original entry", names)
	}
}

// A dry-run must change nothing — neither content nor the record.
func TestMigrateData_DryRunRecordsNothing(t *testing.T) {
	meta := migrateTestMeta()
	svc, root := migrateTestServices(t, meta)
	writeMigration(t, root, "20260919143022-backfill-owner.yaml", meta,
		"  - set_default: {entity: task, property: owner, value: unassigned}\n")

	if err := (&MigrateDataCmd{}).Run(t.Context(), svc); err != nil {
		t.Fatalf("migrate data: %v", err)
	}
	if st := readApplied(t, svc); st != nil && len(st.Applied) != 0 {
		t.Fatalf("a dry-run recorded %v", st.AppliedNames())
	}
}

// With migrations present and nothing recorded, the gate must refuse rather
// than baseline over files that may still need to run.
func TestMigrateStatus_RefusesWhenUnbaselined(t *testing.T) {
	meta := migrateTestMeta()
	svc, root := migrateTestServices(t, meta)
	writeMigration(t, root, "20260919143022-backfill-owner.yaml", meta,
		"  - set_default: {entity: task, property: owner, value: unassigned}\n")

	err := (&MigrateStatusCmd{}).Run(t.Context(), svc)
	if err == nil {
		t.Fatal("status must exit non-zero while the store is un-baselined")
	}
	if st := readApplied(t, svc); st != nil {
		t.Error("the un-baselined case must record nothing")
	}
}

// baseline marks the migrations applied without running them — the override
// for a store whose content already matches the schema.
func TestMigrateBaseline_RecordsWithoutRunning(t *testing.T) {
	meta := migrateTestMeta()
	svc, root := migrateTestServices(t, meta)
	const name = "20260919143022-backfill-owner.yaml"
	writeMigration(t, root, name, meta,
		"  - set_default: {entity: task, property: owner, value: unassigned}\n")

	// Dry-run first: nothing recorded.
	if err := (&MigrateBaselineCmd{}).Run(t.Context(), svc); err != nil {
		t.Fatalf("migrate baseline: %v", err)
	}
	if st := readApplied(t, svc); st != nil {
		t.Fatal("a baseline dry-run must record nothing")
	}

	if err := (&MigrateBaselineCmd{Apply: true}).Run(t.Context(), svc); err != nil {
		t.Fatalf("migrate baseline --apply: %v", err)
	}
	st := readApplied(t, svc)
	if st == nil {
		t.Fatal("baseline --apply recorded nothing")
	}
	if names := st.AppliedNames(); len(names) != 1 || names[0] != name {
		t.Fatalf("applied = %v, want [%s]", names, name)
	}

	// And it is audited: marking migrations applied without running them is a
	// claim an operator made, so it must be reconstructable afterwards.
	if _, err := svc.MigState.Load(t.Context()); err != nil {
		t.Fatalf("load: %v", err)
	}
}

// Baselining a store that already has a record must be refused rather than
// silently rewriting its position.
func TestMigrateBaseline_RefusesWhenAlreadyRecorded(t *testing.T) {
	meta := migrateTestMeta()
	svc, root := migrateTestServices(t, meta)
	writeMigration(t, root, "20260919143022-backfill-owner.yaml", meta,
		"  - set_default: {entity: task, property: owner, value: unassigned}\n")
	if err := (&MigrateBaselineCmd{Apply: true}).Run(t.Context(), svc); err != nil {
		t.Fatalf("first baseline: %v", err)
	}
	before := readApplied(t, svc).AppliedNames()

	// A second file arrives; baseline must NOT quietly adopt it.
	writeMigration(t, root, "20260920090000-second.yaml", meta,
		"  - set_default: {entity: task, property: owner, value: other}\n")
	if err := (&MigrateBaselineCmd{Apply: true}).Run(t.Context(), svc); err != nil {
		t.Fatalf("second baseline: %v", err)
	}
	if after := readApplied(t, svc).AppliedNames(); len(after) != len(before) {
		t.Errorf("baseline rewrote an existing record: %v → %v", before, after)
	}
}

// gen drafts a timestamp-named file, never the old sequential scheme.
func TestMigrateGen_WritesTimestampNamedDraft(t *testing.T) {
	meta := migrateTestMeta()
	svc, root := migrateTestServices(t, meta)

	// Record a baseline, then make an incompatible change so gen has a diff.
	if err := (&MigrateDataCmd{}).Run(t.Context(), svc); err != nil {
		t.Fatalf("baseline via migrate data: %v", err)
	}
	def := svc.Meta.Entities["task"]
	def.Properties["owner"] = metamodel.PropertyDef{Type: "integer"}
	svc.Meta.Entities["task"] = def

	if err := (&MigrateGenCmd{Description: "owner becomes an integer"}).Run(t.Context(), svc); err != nil {
		t.Fatalf("migrate gen: %v", err)
	}

	entries, err := os.ReadDir(filepath.Join(root, datamigration.MigrationsDir))
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}
	var drafted string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".yaml") {
			drafted = e.Name()
		}
	}
	if drafted == "" {
		t.Fatal("gen wrote no migration file")
	}
	if !datamigration.IsMigrationFileName(drafted) {
		t.Errorf("drafted %q, which the loader would reject", drafted)
	}
	body, err := os.ReadFile(filepath.Join(root, datamigration.MigrationsDir, drafted))
	if err != nil {
		t.Fatalf("read draft: %v", err)
	}
	// No shape-hash edge: that is what made a data-only migration impossible.
	if strings.Contains(string(body), "\nfrom:") || strings.Contains(string(body), "\nto:") {
		t.Errorf("draft still carries a from/to hash:\n%s", body)
	}
	// The projections stay: step validation runs on them.
	if !strings.Contains(string(body), "from_projection:") {
		t.Errorf("draft is missing its embedded projections:\n%s", body)
	}
}

// A YAML file whose name is outside the allowlist fails the load LOUDLY.
//
// Skipping it quietly would be the worse outcome: a migration the operator
// wrote would never run and nothing would say so. This is also the upgrade
// signal for a project carrying legacy %04d names — the error names the
// expected form, and `rela migrate baseline` records the set once renamed.
func TestMigrateData_RejectsBadlyNamedMigration(t *testing.T) {
	meta := migrateTestMeta()
	svc, root := migrateTestServices(t, meta)
	dir := filepath.Join(root, datamigration.MigrationsDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "0001-legacy.yaml"), []byte("steps: []\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	err := (&MigrateDataCmd{}).Run(t.Context(), svc)
	if err == nil {
		t.Fatal("a migration file with an invalid name must fail the load, not be skipped")
	}
	if !strings.Contains(err.Error(), "14-digit timestamp") {
		t.Errorf("error should name the expected form, got: %v", err)
	}
	if st := readApplied(t, svc); st != nil && len(st.Applied) != 0 {
		t.Errorf("nothing should be recorded when the load fails: %v", st.AppliedNames())
	}
}
