package projectsetup_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/projectsetup"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// TKT-76JP2A: acl.yaml joins the migrated file set. These tests drive the
// real runner over a real temp project, because the unit tests on the
// migration itself cannot catch a file that is never handed to it.

const aclTestMetamodel = `entities:
  ticket:
    id_type: sequential
    properties:
      title:
        type: string
`

// newACLProject writes a minimal project. When aclYAML is empty, no
// acl.yaml is created — the "policy-less project" case.
func newACLProject(t *testing.T, aclYAML string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "metamodel.yaml"), []byte(aclTestMetamodel), 0o600); err != nil {
		t.Fatalf("write metamodel: %v", err)
	}
	if aclYAML != "" {
		if err := os.WriteFile(filepath.Join(dir, "acl.yaml"), []byte(aclYAML), 0o600); err != nil {
			t.Fatalf("write acl.yaml: %v", err)
		}
	}
	return dir
}

// TestMigrate_GrantsSchedulerInExistingACL is the end-to-end repair: an
// operator's policy that never mentioned the scheduler gains the grant.
func TestMigrate_GrantsSchedulerInExistingACL(t *testing.T) {
	dir := newACLProject(t, "# my policy\nroles:\n  viewer:\n    read: [ticket]\nassignments:\n  alice: viewer\n")
	fs := storage.NewSafeFS(storage.NewOsFS())

	if _, err := projectsetup.MigrateWithFS(dir, fs); err != nil {
		t.Fatalf("MigrateWithFS: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "acl.yaml"))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	got := string(raw)
	if !strings.Contains(got, "# my policy") {
		t.Errorf("operator's comment was lost:\n%s", got)
	}

	var policy acl.Policy
	if err := yaml.Unmarshal(raw, &policy); err != nil {
		t.Fatalf("migrated acl.yaml does not parse: %v\n%s", err, got)
	}
	if err := policy.Validate(); err != nil {
		t.Fatalf("migrated acl.yaml fails Validate: %v\n%s", err, got)
	}
	if _, ok := policy.Assignments[principal.UserScheduler]; !ok {
		t.Errorf("no grant for %q:\n%s", principal.UserScheduler, got)
	}
	if policy.Assignments["alice"] != "viewer" {
		t.Errorf("existing assignment lost:\n%s", got)
	}
}

// TestMigrate_LeavesPolicylessProjectAlone is AC3 and the guard on
// RR-SVQ5HE: a project with no acl.yaml runs on NopACL, where scheduled
// tasks already read fine. Creating a policy would flip every principal
// (humans, CLI, MCP) to deny-by-default. It must stay absent.
func TestMigrate_LeavesPolicylessProjectAlone(t *testing.T) {
	dir := newACLProject(t, "")
	fs := storage.NewSafeFS(storage.NewOsFS())

	if _, err := projectsetup.MigrateWithFS(dir, fs); err != nil {
		t.Fatalf("MigrateWithFS: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "acl.yaml")); !os.IsNotExist(err) {
		raw, _ := os.ReadFile(filepath.Join(dir, "acl.yaml"))
		t.Fatalf("migration CREATED acl.yaml in a policy-less project "+
			"(this denies reads to every principal); content:\n%s", raw)
	}
}

// TestMigrate_ACLIsIdempotent: running migrate twice must not double up.
func TestMigrate_ACLIsIdempotent(t *testing.T) {
	dir := newACLProject(t, "roles:\n  viewer:\n    read: [ticket]\nassignments:\n  alice: viewer\n")
	fs := storage.NewSafeFS(storage.NewOsFS())
	path := filepath.Join(dir, "acl.yaml")

	if _, err := projectsetup.MigrateWithFS(dir, fs); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if _, migrateErr := projectsetup.MigrateWithFS(dir, fs); migrateErr != nil {
		t.Fatalf("second migrate: %v", migrateErr)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if !bytes.Equal(first, second) {
		t.Errorf("second migrate changed the file:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
}

// TestDetectMigrations_ReportsACLGrant pins that `rela migrate --check`
// surfaces this before it is applied.
func TestDetectMigrations_ReportsACLGrant(t *testing.T) {
	dir := newACLProject(t, "roles:\n  viewer:\n    read: [ticket]\nassignments:\n  alice: viewer\n")
	fs := storage.NewSafeFS(storage.NewOsFS())

	detections, err := projectsetup.DetectMigrationsWithFS(dir, fs)
	if err != nil {
		t.Fatalf("DetectMigrationsWithFS: %v", err)
	}
	for _, d := range detections {
		if d.File.Name != "acl.yaml" {
			continue
		}
		for _, m := range d.Migrations {
			if m.Migration.Name() == "acl-scheduler-grant" {
				return
			}
		}
	}
	t.Errorf("acl-scheduler-grant not reported by --check; detections=%+v", detections)
}

// TestMigrate_MalformedACLDoesNotClobber: a broken policy must fail the
// migration and leave the operator's bytes on disk untouched.
func TestMigrate_MalformedACLDoesNotClobber(t *testing.T) {
	const broken = "roles:\n  viewer:\n    read: [ticket\nassignments:\n"
	dir := newACLProject(t, broken)
	fs := storage.NewSafeFS(storage.NewOsFS())

	if _, err := projectsetup.MigrateWithFS(dir, fs); err == nil {
		t.Error("malformed acl.yaml migrated without error")
	}

	raw, err := os.ReadFile(filepath.Join(dir, "acl.yaml"))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(raw) != broken {
		t.Errorf("malformed file was rewritten:\n--- want ---\n%s\n--- got ---\n%s", broken, raw)
	}
}
