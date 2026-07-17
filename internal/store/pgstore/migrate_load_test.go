package pgstore

import "testing"

// knownDuplicateVersions is the set of migration version prefixes that are
// already shared by two files in a released schema (BUG-TY2XQC: 0003_sync.sql
// and 0003_attachments_per_file_pk.sql). Renumbering either would change the
// version an already-migrated deployment recorded, so the duplicate is
// grandfathered here rather than fixed in this ticket. New migrations must NOT
// add to this set.
var knownDuplicateVersions = map[int]bool{3: true}

// TestLoadMigrationsIntegrity guards the embedded migration set without a
// database: every file parses to a version, files are in sorted version order,
// each has SQL, and — crucially — no NEW file introduces a duplicate version
// prefix. A duplicate prefix is a real hazard: parseMigrationVersion keys on the
// integer prefix and Migrate skips version <= current, so a second file at an
// already-applied version is silently never run on an existing database.
func TestLoadMigrationsIntegrity(t *testing.T) {
	migs, err := loadMigrations()
	if err != nil {
		t.Fatalf("loadMigrations: %v", err)
	}
	if len(migs) == 0 {
		t.Fatal("no migrations loaded")
	}

	seen := make(map[int]string, len(migs))
	prev := -1
	for _, m := range migs {
		if existing, dup := seen[m.version]; dup && !knownDuplicateVersions[m.version] {
			t.Errorf("NEW duplicate migration version %d: %q and %q share a prefix "+
				"(each version prefix must be unique — see BUG-TY2XQC)", m.version, existing, m.name)
		}
		seen[m.version] = m.name
		if m.version < prev {
			t.Errorf("migration %q version %d is out of sorted order (prev %d)", m.name, m.version, prev)
		}
		prev = m.version
		if m.sql == "" {
			t.Errorf("migration %q has empty SQL", m.name)
		}
	}
}
