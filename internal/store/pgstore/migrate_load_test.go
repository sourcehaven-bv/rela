package pgstore

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

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

// releasedMigrations pins the sha256 of every migration that has shipped in a
// release. Migrate records only the version number, so an edit to a released
// file never reaches a database already past it, and fresh and upgraded
// deployments silently diverge.
//
// When cutting a release, add each new migration file to this map with the
// output of `shasum -a 256 internal/store/pgstore/migrations/<file>`. Files
// above the last release are not listed, so they may change until it ships.
var releasedMigrations = map[string]string{
	"0001_init.sql":                    "34d061a7b2d521558760b0140c62fdfd0917b7ad1fd3eec2a3860e405688b14f",
	"0002_relations_composite_idx.sql": "4e56cad876a44c1a4b0d52f467cec8bd796341344ae9b0dc362c08629fcd31d7",
	"0003_attachments_per_file_pk.sql": "3c45314fa1a13eadbd8bb379bee5a2a5ef0d807aa6844496d81680af5b67b181",
	"0003_sync.sql":                    "a0d58d4e753e1fb14b9fb4db45ea6ef50c4062d9d96a7c5c83a5c7711472a2c2",
	"0004_versions.sql":                "3ff7ebcf0f10ca4c767ade392a02a60486ca333341dc13cd0dc4509f5baf4005",
	"0005_relation_versions.sql":       "b72232ad7ae97d6cd3b41f864b56e8643476a9a46a439f3aa724eedbdcb2f7aa",
	"0006_last_edited_by.sql":          "fc980ca5cddee9f8bba73e104c5fbbfcc1945ecf91eeeded82b7260c23103476",
	"0007_entity_id_case_identity.sql": "0986238bbb159401ff6b8faef5ccab323eb023317a07a06b18e6d4025a4d344d",
	"0008_state_kv.sql":                "868bc3ac982a37eb0f1c5d06a4fa57dac2badd683ad752d6923ba46af77d7b54",
	"0009_next_action_state.sql":       "fe20306b1f2dc249fd6f929121d09fd7da58a7ffb33a2f57db1cfd3fe4ca66c3",
	"0010_type_watermark.sql":          "536f738ac6c5f8abbcdb92cbdc5ac13811f1ca7e0df4c80709273d882c71fb9b",
	"0011_content_states.sql":          "8d509d6f8dede4bc22ad5f48b4eb5ca080222929d53340bdd1ec5c56b1a5960d",
	"0012_per_state_versions.sql":      "62dde0e8a16ba90666ecdac02201a7300e5f2c78831568ec317ccf27a4719283",
	"0013_write_origin.sql":            "665b7109076b0e72b065aa9e09edee6eccbb90679c08d97054a3617cc0d911cf",
	"0014_read_indexes.sql":            "a87ee95ef88103e7d6839210ced08966ae0f21d8949e4a33a14c20ef6af3915b",
	"0015_comments.sql":                "d48054dae377c67887b3bd9706c3ef8dcd88a788b91bb28f8fe2b89664344c64",
	"0016_migration_state.sql":         "345ede74e332489ed67194c46a9fb51aa6e452768cbc87b010660df4549e456f",
	"0017_scheduler_runs.sql":          "51751db83895488fce443a5f882b121977b3e1ca30298b3cb75a9e044b333dfb",
	"0018_soft_delete.sql":             "67453aa857b1d0aa32847cff4a2a0f9a5aaa4d8145a45cc29e1774214811a011",
	"0019_face_read_indexes.sql":       "a6cf618a70ca9ff43847228d42dfb21508075a92be5493f6be298d92cda01dce",
	"0020_live_content_hash.sql":       "2688acf32783b757aabd5ca037b27f974bddde5f88e73112699d57e28be67361",
	"0021_unhashed_rows_index.sql":     "4979c9242fd6c0e290cc18ab155608a946a31d349b3d0035cc18c44d66d26f17",
	"0022_version_tags.sql":            "ec9594535fd60c62e875a72b1c03ea3996c7a917838903d64e2925240a1297b2",
	"0023_piles.sql":                   "06c00bccccd677af5fbeb649c4655e53a8147cde588b2b524c886af59cafe3f4",
	"0024_mention_targets.sql":         "35070cb4375478b1a14010850a52b04e7cf3313aa88312dfd4a5b2355f9b1875",
	"0025_leases.sql":                  "53a0801f5b21a4adb92c2a5eb36f4f2426ed44588386cc2d740bb42a641d4552",
	"0026_channels.sql":                "7541a69fdcb69ad35b6129cbef36fc9b52649e578ddbb132f01ecea16a62045a",
}

// TestReleasedMigrationsUnchanged fails when a migration that shipped in a
// release is edited or removed. A schema change goes in a new migration.
func TestReleasedMigrationsUnchanged(t *testing.T) {
	migs, err := loadMigrations()
	if err != nil {
		t.Fatalf("loadMigrations: %v", err)
	}
	got := make(map[string]string, len(migs))
	for _, m := range migs {
		sum := sha256.Sum256([]byte(m.sql))
		got[m.name] = hex.EncodeToString(sum[:])
	}
	for name, want := range releasedMigrations {
		sum, ok := got[name]
		switch {
		case !ok:
			t.Errorf("migration %s shipped in a release and is missing; restore it", name)
		case sum != want:
			t.Errorf("migration %s shipped in a release and must not change; add a new migration instead", name)
		}
	}
}
