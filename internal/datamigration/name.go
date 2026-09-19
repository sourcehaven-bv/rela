package datamigration

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// maxNameLen caps a migration filename well below the 255-byte limit every
// mainstream filesystem imposes, so a name that validates here can always be
// created. The slug carries a human description, not data; 128 bytes is
// generous for that and leaves no room for a name engineered to sit exactly on
// a filesystem boundary.
const maxNameLen = 128

// nameRe is the ALLOWLIST for a migration filename: a 14-digit timestamp, a
// hyphen, then a lowercase-alphanumeric slug whose words are hyphen-separated.
//
// Lowercase-only is load-bearing rather than stylistic. The applied-list
// records names and [LoadDir] lists them from a directory, and the two
// populations are compared by equality — so a filesystem that folds case
// (macOS and Windows, by default) would otherwise let `0001-Foo.yaml` and
// `0001-foo.yaml` be ONE file but TWO applied-list entries, making a migration
// re-run or appear applied when it is not. Restricting the alphabet to
// lowercase ASCII removes the divergence instead of trying to compensate for
// it, and forecloses the same class of collision between Unicode NFC and NFD
// spellings of one name.
var nameRe = regexp.MustCompile(`^\d{14}-[a-z0-9]+(-[a-z0-9]+)*\.ya?ml$`)

// MigrationName is a validated migration filename.
//
// The field is unexported and [ParseMigrationName] is the only way in, so a
// value of this type has passed the allowlist by construction — the
// param-contract convention rela uses wherever a bare string would otherwise
// carry an unchecked precondition (compare [principal.Principal], which keeps
// its roles unexported for the same reason).
//
// That matters here because a name reaches a path join. Names round-trip
// through `migrations/applied.json`, which is committed, hand-editable and
// resolved by whoever fixes a merge conflict — so they are operator-authored
// but NOT trustworthy as written.
type MigrationName struct {
	name string
}

// String returns the filename.
func (m MigrationName) String() string { return m.name }

// IsZero reports whether m is the zero value (no name).
func (m MigrationName) IsZero() bool { return m.name == "" }

// ParseMigrationName validates s against the allowlist and returns it as a
// [MigrationName].
//
// Rejection is total: there is no sanitizing path that strips offending
// characters and continues, because a name that had to be repaired no longer
// matches the file the operator meant. Every error names the rule that failed
// so a bad hand-edit says what to fix.
func ParseMigrationName(s string) (MigrationName, error) {
	switch {
	case s == "":
		return MigrationName{}, errors.New("datamigration: migration name is empty")
	case len(s) > maxNameLen:
		return MigrationName{}, fmt.Errorf(
			"datamigration: migration name %q is %d bytes, over the %d-byte limit", s, len(s), maxNameLen)
	case !nameRe.MatchString(s):
		return MigrationName{}, fmt.Errorf(
			"datamigration: migration name %q is not of the form <14-digit timestamp>-<lowercase-slug>.yaml "+
				"(for example 20260919143022-backfill-owner.yaml)", s)
	}
	return MigrationName{name: s}, nil
}

// IsMigrationFileName reports whether s is a well-formed migration filename.
// [LoadDir] uses it to decide which directory entries are migrations at all,
// so the directory listing and the applied-list are drawn from one alphabet
// and comparing them means something.
func IsMigrationFileName(s string) bool {
	return len(s) <= maxNameLen && nameRe.MatchString(s)
}

// NewMigrationFileName builds a migration filename from a timestamp and a
// free-text description, slugifying the description so the result validates.
//
// Nil: never returns an empty name — a description that slugifies to nothing
// falls back to "migration", because a file still needs a name.
func NewMigrationFileName(stamp, description string) (MigrationName, error) {
	return ParseMigrationName(stamp + "-" + slugify(description) + ".yaml")
}

// slugify reduces free text to the lowercase-hyphen alphabet [nameRe] accepts:
// runs of anything else collapse to a single hyphen, and the result is trimmed
// so it can neither start nor end with one.
func slugify(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			dash = false
		default:
			// Collapse a run of separators to one hyphen, and never emit a
			// leading one — both would fail the allowlist.
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	// Truncating can strip back to a trailing hyphen, so trim again after.
	if len(out) > maxNameLen-len("20260919143022-.yaml") {
		out = strings.Trim(out[:maxNameLen-len("20260919143022-.yaml")], "-")
	}
	if out == "" {
		return "migration"
	}
	return out
}
