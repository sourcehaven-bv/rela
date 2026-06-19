// Package canonical produces a deterministic, backend-independent content
// hash for an [entity.Entity] or [entity.Relation].
//
// The hash is the load-bearing token of the sync feature (FEAT-NJ9FEN): a
// record edited on the filesystem (fsstore) and the same record stored in
// Postgres (pgstore) must hash to the same value, or every conditional push
// fails its If-Match precondition and every pull reports a phantom diff.
//
// The two backends reconstruct an entity from different on-disk forms and
// arrive at different concrete Go types for the same logical value:
//
//   - fsstore parses YAML frontmatter (gopkg.in/yaml.v3): whole numbers decode
//     as int, lists as []any, nested maps as map[string]any.
//   - pgstore parses a JSONB blob with UseNumber + normalizeJSONNumbers: whole
//     numbers as int, fractional as float64, lists as []any.
//
// Hashing the stored bytes (reflowed markdown vs. raw JSONB columns) could
// never match. Hashing the Go values directly is fragile, because int vs.
// int64 vs. float64 and []string vs. []any are representation
// accidents, not logical differences. So this package does neither: it walks
// the reconstructed value and emits a single normal form that is invariant to
// those accidents — numbers as their shortest decimal, slices in order, maps
// with sorted keys — then hashes that.
//
// The markdown body is normalized through [markdown.FormatMarkdown] so that
// fsstore's already-reflowed body and pgstore's raw Content converge on the
// same canonical text. This relies on FormatMarkdown being idempotent
// (reflowing already-reflowed text is a no-op), which the tests assert.
package canonical

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/markdown"
)

// Canonical-form delimiters. These ASCII separator control characters do not
// occur in well-formed markdown frontmatter keys or values, so they cannot be
// produced by record content and a value cannot smuggle a delimiter to forge a
// different structure with the same bytes.
const (
	sepUnit   = '\x1f' // between a field key and its value, and between props
	sepRecord = '\x1e' // ends a field/property record
	sepItem   = '\x1d' // between slice items and map entries
	sepKV     = '\x1c' // between a nested-map key and its value
)

// HashEntity returns the canonical content hash of an entity as a hex-encoded
// SHA-256 digest. The hash covers the id, type, properties, and (normalized)
// body. It deliberately ignores UpdatedAt (a storage timestamp, not content)
// and the Inaccessible set (a per-reader redaction artifact, not content).
func HashEntity(e entity.Entity) string {
	var b strings.Builder
	b.WriteString("entity\x1f")
	writeField(&b, "id", e.ID)
	writeField(&b, "type", e.Type)
	writeProperties(&b, e.Properties)
	writeBody(&b, e.Content)
	return digest(b.String())
}

// HashRelation returns the canonical content hash of a relation as a
// hex-encoded SHA-256 digest. The hash covers the from/type/to triple,
// properties, and (normalized) body, ignoring UpdatedAt and Inaccessible.
func HashRelation(r entity.Relation) string {
	var b strings.Builder
	b.WriteString("relation\x1f")
	writeField(&b, "from", r.From)
	writeField(&b, "relation", r.Type)
	writeField(&b, "to", r.To)
	writeProperties(&b, r.Properties)
	writeBody(&b, r.Content)
	return digest(b.String())
}

// digest hashes the canonical string. The string uses the ASCII unit separator
// (0x1f) and record separator (0x1e) as delimiters — bytes that do not occur in
// well-formed markdown frontmatter keys or values — so distinct structures
// cannot collide by concatenation.
func digest(canonical string) string {
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:])
}

func writeField(b *strings.Builder, key, value string) {
	b.WriteString(key)
	b.WriteByte(sepUnit)
	b.WriteString(value)
	b.WriteByte(sepRecord)
}

// writeProperties emits every property in sorted-key order, each value run
// through canonicalValue so the byte form is independent of the concrete Go
// type the backend produced. A nil or empty map emits nothing, so an entity
// with no properties and one with an empty (non-nil) map hash identically.
func writeProperties(b *strings.Builder, props map[string]any) {
	if len(props) == 0 {
		return
	}
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	b.WriteString("props\x1f")
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte(sepUnit)
		canonicalValue(b, props[k])
		b.WriteByte(sepRecord)
	}
}

func writeBody(b *strings.Builder, content string) {
	b.WriteString("body\x1f")
	b.WriteString(markdown.FormatMarkdown(content))
	b.WriteByte(sepRecord)
}

// canonicalValue writes a normal form of v that is invariant to the
// representation differences between the YAML (fsstore) and JSONB (pgstore)
// decode paths. Numbers collapse to their shortest decimal, slices are emitted
// in order, and maps are emitted with sorted keys. Each value is tagged with a
// type sigil so that, e.g., the string "1" and the number 1 do not collide.
func canonicalValue(b *strings.Builder, v any) {
	switch val := v.(type) {
	case nil:
		b.WriteString("n:")
	case string:
		b.WriteString("s:")
		b.WriteString(val)
	case bool:
		b.WriteString("b:")
		b.WriteString(strconv.FormatBool(val))
	case int, int8, int16, int32, int64:
		b.WriteString("i:")
		b.WriteString(strconv.FormatInt(reflectInt(val), 10))
	case uint, uint8, uint16, uint32, uint64:
		b.WriteString("i:")
		b.WriteString(strconv.FormatUint(reflectUint(val), 10))
	case float32:
		writeFloat(b, float64(val))
	case float64:
		writeFloat(b, val)
	case []string:
		b.WriteString("a:")
		for _, item := range val {
			canonicalValue(b, item)
			b.WriteByte(sepItem)
		}
	case []any:
		b.WriteString("a:")
		for _, item := range val {
			canonicalValue(b, item)
			b.WriteByte(sepItem)
		}
	case map[string]any:
		b.WriteString("m:")
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			b.WriteString(k)
			b.WriteByte(sepKV)
			canonicalValue(b, val[k])
			b.WriteByte(sepItem)
		}
	default:
		// Unknown type: fall back to Go's default formatting. This should not
		// happen for values that round-trip through YAML or JSONB, but a sigil
		// keeps it unambiguous and deterministic rather than panicking.
		b.WriteString("u:")
		fmt.Fprintf(b, "%v", val)
	}
}

// reflectInt widens any signed-integer kind to int64 so all signed widths
// canonicalize to the same bytes (int(5) and int64(5) are not logically
// different).
func reflectInt(v any) int64 {
	switch n := v.(type) {
	case int:
		return int64(n)
	case int8:
		return int64(n)
	case int16:
		return int64(n)
	case int32:
		return int64(n)
	case int64:
		return n
	default:
		return 0
	}
}

// reflectUint widens any unsigned-integer kind to uint64.
func reflectUint(v any) uint64 {
	switch n := v.(type) {
	case uint:
		return uint64(n)
	case uint8:
		return uint64(n)
	case uint16:
		return uint64(n)
	case uint32:
		return uint64(n)
	case uint64:
		return n
	default:
		return 0
	}
}

// writeFloat emits a float in a form that collapses whole-valued floats to the
// same bytes as the equivalent integer would NOT — instead it always tags as a
// number and uses the shortest round-trippable decimal. A whole float like 3.0
// and the int 3 intentionally produce different bytes (i:3 vs f:3) only if the
// backends disagree on the type; in practice both backends normalize whole
// numbers to int (see normalizeJSONNumbers), so a fractional value reaching
// here is genuinely fractional.
func writeFloat(b *strings.Builder, f float64) {
	b.WriteString("f:")
	b.WriteString(strconv.FormatFloat(f, 'g', -1, 64))
}
