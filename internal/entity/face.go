package entity

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Face identifies one content state of an entity (FEAT-9CD2MX,
// TKT-DOFYR1). Its value is the CANONICAL serialized coordinate —
// "draft" today, a codec-canonicalized multi-axis form like "nl+draft"
// when axes arrive (§11 of the design doc reserves '+'; axis ordering
// will be pinned by the codec so the canonical form stays unique). The
// zero value means the default state / a faceless entity, which is
// why a faceless project never sees this type at all.
//
// Two rules are load-bearing; both exist so multi-axis later becomes a
// resolver feature instead of a storage migration:
//
//   - The codec below ([ParseFace], [ParseStateRef]) is the ONLY
//     constructor from external input. Nothing else builds a Face by
//     string conversion or concatenation; internal code passes the value
//     around opaquely. Canonical-form uniqueness is what licenses the
//     next rule, plus == comparison, one pg text column, and one
//     frontmatter key.
//
//   - Stores only ever EQUALITY-MATCH a Face — never parse or
//     inspect its contents. Worlds (Step 2) compile to sets of concrete
//     coordinates before they reach a store, so store matching never
//     changes shape. The storetest suite pins this by round-tripping an
//     unusual-but-canonical value unchanged.
type Face string

// IsDefault reports whether p addresses the default state (the zero
// value). A faceless entity is its own default state (§2.1).
func (p Face) IsDefault() bool { return p == "" }

// String returns the canonical serialized coordinate. It exists for
// boundaries — filenames, the pg column, change-feed payloads — which
// are exactly where the serialized form is allowed to appear (§3.2).
func (p Face) String() string { return string(p) }

// StateRefSeparator joins a base entity ID and a face in the
// boundary serialization: "PAGE-1@draft". '@' is filename-legal,
// excluded from the entity-ID grammar (ValidateID rejects it), and
// distinct from the relation-key separator "--" and the pgstore
// change-feed field separator '\x1f'.
const StateRefSeparator = "@"

// facePattern is the grammar for a single face coordinate:
// lowercase alphanumeric runs joined by single hyphens. Deliberately
// narrower than the entity-ID grammar, and — like ValidateID's
// no-consecutive-hyphens rule — it must never admit "--": the face
// serializes into the FROM slot of relation keys ("FROM--TYPE--TO"),
// so a face containing the separator would make relation filenames
// ambiguous to parse. Load-bearing for the storage format.
var facePattern = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

// ParseFace validates and canonicalizes a bare face coordinate
// from external input (URL, CLI, acl.yaml, filename, stored column).
// It is grammar-only: whether the face is DECLARED by the metamodel
// is validated at metamodel load, never here — this package must not
// know the metamodel.
//
// v1 ships a single axis, so canonicalization is the identity; '+' is
// reserved for future multi-axis coordinates and rejected with a
// distinct error so a newer project's data fails loudly on an older
// binary rather than being misread.
func ParseFace(s string) (Face, error) {
	if s == "" {
		return "", errors.New("empty face")
	}
	if strings.Contains(s, "+") {
		return "", fmt.Errorf("multi-axis face coordinates are not supported yet: %q", s)
	}
	if !facePattern.MatchString(s) {
		return "", fmt.Errorf("invalid face %q: must be lowercase alphanumeric runs joined by single hyphens", s)
	}
	return Face(s), nil
}

// ParseStateRef parses the boundary serialization of a state reference:
// "PAGE-1" (default state) or "PAGE-1@draft". At most one separator; a
// state cannot have a state. The base ID goes through [ValidateID] —
// the single ID grammar — and the face through [ParseFace].
func ParseStateRef(s string) (id string, p Face, err error) {
	base, ptr, found := strings.Cut(s, StateRefSeparator)
	if idErr := ValidateID(base); idErr != nil {
		return "", "", fmt.Errorf("invalid state reference %q: %w", s, idErr)
	}
	if !found {
		return base, "", nil
	}
	if strings.Contains(ptr, StateRefSeparator) {
		return "", "", fmt.Errorf("invalid state reference %q: more than one %q", s, StateRefSeparator)
	}
	p, err = ParseFace(ptr)
	if err != nil {
		return "", "", fmt.Errorf("invalid state reference %q: %w", s, err)
	}
	return base, p, nil
}

// FormatStateRef renders the boundary serialization of (id, p):
// the bare id for the default state, "id@face" otherwise. It never
// emits an empty face — the default state's serialization IS the
// bare id. The id is assumed valid and p canonical (both only exist
// via their parse functions); Format performs no re-validation.
func FormatStateRef(id string, p Face) string {
	if p.IsDefault() {
		return id
	}
	return id + StateRefSeparator + string(p)
}
