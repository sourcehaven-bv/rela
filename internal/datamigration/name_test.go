package datamigration

import (
	"strings"
	"testing"
)

func TestParseMigrationName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		ok   bool
		why  string
	}{
		{name: "canonical", in: "20260919143022-backfill-owner.yaml", ok: true},
		{name: "yml suffix", in: "20260919143022-backfill.yml", ok: true},
		{name: "single word slug", in: "20260919143022-backfill.yaml", ok: true},
		{name: "digits in slug", in: "20260919143022-fix-bug-123.yaml", ok: true},

		{name: "empty", in: "", why: "a name is required"},
		{name: "path traversal", in: "../../etc/passwd", why: "reaches a path join"},
		{name: "nested path", in: "sub/20260919143022-x.yaml", why: "must be one path segment"},
		{name: "absolute path", in: "/20260919143022-x.yaml", why: "must be one path segment"},
		{
			name: "uppercase",
			in:   "20260919143022-Backfill.yaml",
			why:  "a case-folding filesystem would make this one file but two applied-list entries",
		},
		{name: "no timestamp", in: "backfill-owner.yaml", why: "ordering depends on the timestamp prefix"},
		{name: "short timestamp", in: "2026091914302-x.yaml", why: "timestamp must be exactly 14 digits"},
		{name: "long timestamp", in: "202609191430221-x.yaml", why: "timestamp must be exactly 14 digits"},
		{name: "no slug", in: "20260919143022-.yaml", why: "slug must not be empty"},
		{name: "missing slug and hyphen", in: "20260919143022.yaml", why: "slug is required"},
		{name: "double hyphen", in: "20260919143022-a--b.yaml", why: "hyphens separate words, never doubled"},
		{name: "trailing hyphen", in: "20260919143022-a-.yaml", why: "slug must not end with a hyphen"},
		{name: "wrong extension", in: "20260919143022-x.json", why: "migrations are yaml"},
		{name: "no extension", in: "20260919143022-x", why: "migrations are yaml"},
		{name: "underscore", in: "20260919143022-a_b.yaml", why: "underscore is outside the allowlist"},
		{name: "space", in: "20260919143022-a b.yaml", why: "space is outside the allowlist"},
		{name: "unicode", in: "20260919143022-café.yaml", why: "NFC and NFD would be two names for one file"},
		{name: "null byte", in: "20260919143022-a\x00b.yaml", why: "null byte truncates in syscalls"},
		{name: "newline", in: "20260919143022-a\nb.yaml", why: "newline is outside the allowlist"},
		{
			name: "over length",
			in:   "20260919143022-" + strings.Repeat("a", 200) + ".yaml",
			why:  "must stay under the filesystem name limit",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseMigrationName(tc.in)
			if tc.ok {
				if err != nil {
					t.Fatalf("ParseMigrationName(%q) = error %v, want ok", tc.in, err)
				}
				if got.String() != tc.in {
					t.Errorf("String() = %q, want %q", got.String(), tc.in)
				}
				return
			}
			if err == nil {
				t.Fatalf("ParseMigrationName(%q) accepted the name; it must be rejected: %s", tc.in, tc.why)
			}
			// A rejected name must not leak out as a usable value.
			if !got.IsZero() {
				t.Errorf("a rejected name must return the zero MigrationName, got %q", got.String())
			}
		})
	}
}

// IsMigrationFileName decides which directory entries are migrations at all,
// so it must agree with ParseMigrationName exactly. Any divergence would let a
// file be listed but not recordable (or the reverse), and the applied-list
// comparison would stop meaning anything.
func TestIsMigrationFileName_AgreesWithParse(t *testing.T) {
	for _, in := range []string{
		"20260919143022-backfill-owner.yaml",
		"20260919143022-x.yml",
		"applied.json",
		"../escape.yaml",
		"0001-legacy-name.yaml",
		"20260919143022-Backfill.yaml",
		"",
	} {
		_, err := ParseMigrationName(in)
		if got, want := IsMigrationFileName(in), err == nil; got != want {
			t.Errorf("IsMigrationFileName(%q) = %v, but ParseMigrationName ok = %v", in, got, want)
		}
	}
}

// The committed applied.json sits in the same directory as the migrations, so
// the listing must not mistake it for one.
func TestIsMigrationFileName_ExcludesTheAppliedList(t *testing.T) {
	if IsMigrationFileName("applied.json") {
		t.Error("applied.json must never be treated as a migration file")
	}
}

// Legacy %04d names stop being valid on purpose: the timestamp prefix is what
// removes the concurrent-PR collision, so a name without one must not sneak
// through.
func TestIsMigrationFileName_RejectsLegacySequentialNames(t *testing.T) {
	if IsMigrationFileName("0001-schema-change.yaml") {
		t.Error("legacy sequential names must not validate under the timestamp scheme")
	}
}

func TestNewMigrationFileName(t *testing.T) {
	tests := []struct {
		name string
		desc string
		want string
	}{
		{name: "simple", desc: "backfill owner", want: "20260919143022-backfill-owner.yaml"},
		{name: "already a slug", desc: "backfill-owner", want: "20260919143022-backfill-owner.yaml"},
		{name: "uppercase folded", desc: "Backfill Owner", want: "20260919143022-backfill-owner.yaml"},
		{name: "punctuation collapsed", desc: "status: open -> todo!", want: "20260919143022-status-open-todo.yaml"},
		{name: "runs collapsed", desc: "a   b", want: "20260919143022-a-b.yaml"},
		{name: "leading junk trimmed", desc: "  --rename--  ", want: "20260919143022-rename.yaml"},
		{name: "empty falls back", desc: "", want: "20260919143022-migration.yaml"},
		{name: "all punctuation falls back", desc: "!!!", want: "20260919143022-migration.yaml"},
		{name: "unicode dropped", desc: "café update", want: "20260919143022-caf-update.yaml"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewMigrationFileName("20260919143022", tc.desc)
			if err != nil {
				t.Fatalf("NewMigrationFileName(%q) = error %v", tc.desc, err)
			}
			if got.String() != tc.want {
				t.Errorf("NewMigrationFileName(%q) = %q, want %q", tc.desc, got.String(), tc.want)
			}
		})
	}
}

// Whatever a description contains, the generated name must validate — the
// generator must never write a file the loader will then refuse.
func TestNewMigrationFileName_AlwaysProducesAValidName(t *testing.T) {
	for _, desc := range []string{
		"", "!!!", strings.Repeat("very long description ", 40),
		"../../escape", "Ünïcödé", "tabs\tand\nnewlines", "trailing-",
	} {
		got, err := NewMigrationFileName("20260919143022", desc)
		if err != nil {
			t.Fatalf("NewMigrationFileName(%q) = error %v", desc, err)
		}
		if !IsMigrationFileName(got.String()) {
			t.Errorf("NewMigrationFileName(%q) produced %q, which the loader would reject", desc, got.String())
		}
	}
}
