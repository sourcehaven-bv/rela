// Package canonical produces a deterministic, backend-independent content
// hash for an [entity.Entity] or [entity.Relation].
//
// The hash is the load-bearing token of the sync feature (FEAT-NJ9FEN): a
// record edited on the filesystem (fsstore) and the same record stored in
// Postgres (pgstore) must hash to the same value, or every conditional push
// fails its If-Match precondition and every pull reports a phantom diff.
//
// # Why this is hard
//
// The two backends reconstruct an entity from different on-disk forms and hand
// us different concrete Go types for the same logical value:
//
//   - fsstore parses YAML frontmatter (gopkg.in/yaml.v3): whole numbers decode
//     as int, an explicit "2.0" stays float64, dates decode as time.Time, a
//     mapping with non-string keys decodes as map[any]any, large unsigned
//     values as uint64.
//   - pgstore round-trips through JSONB: time.Time becomes an RFC3339 string,
//     "2.0" folds back to int, every map is map[string]any, a uint64 above
//     math.MaxInt64 reads back as a (lossy) float64.
//
// Hashing the stored bytes (reflowed markdown vs. raw JSONB columns) could
// never match. Hashing the Go values directly is fragile, because these are
// representation accidents, not logical differences. So this package does two
// things:
//
//  1. [normalize] folds every value into ONE canonical Go form at a single
//     boundary — whole floats to int, time.Time to its RFC3339 string,
//     map[any]any to map[string]any, uint64 above MaxInt64 to the same lossy
//     float64 pgstore is forced to read. After normalization both backends'
//     values are byte-for-byte the same shape.
//  2. The hash is built by streaming length-prefixed fields into a SHA-256
//     hash (see [writer]). Length prefixes make the encoding unambiguous for
//     arbitrary content: a value cannot smuggle a delimiter to forge a
//     different structure with the same bytes (a real collision risk when the
//     value is user- or LLM-authored and may contain control characters).
//
// The markdown body is normalized through [markdown.FormatMarkdown] so that
// fsstore's reflowed body and pgstore's raw Content converge. This relies on
// FormatMarkdown being idempotent, which the fuzz tests assert.
package canonical

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/markdown"
)

// HashEntity returns the canonical content hash of an entity as a hex-encoded
// SHA-256 digest. The hash covers the id, type, properties, and (normalized)
// body. It deliberately ignores UpdatedAt (a storage timestamp, not content)
// and the Inaccessible set (a per-reader redaction artifact, not content).
func HashEntity(e entity.Entity) string {
	w := newWriter()
	w.tag('E')
	w.field("id", e.ID)
	w.field("type", e.Type)
	w.properties(e.Properties)
	w.body(e.Content)
	return w.sum()
}

// HashRelation returns the canonical content hash of a relation as a
// hex-encoded SHA-256 digest. The hash covers the from/type/to triple,
// properties, and (normalized) body, ignoring UpdatedAt and Inaccessible.
func HashRelation(r entity.Relation) string {
	w := newWriter()
	w.tag('R')
	w.field("from", r.From)
	w.field("relation", r.Type)
	w.field("to", r.To)
	w.properties(r.Properties)
	w.body(r.Content)
	return w.sum()
}

// writer streams a length-prefixed encoding of a record into a SHA-256 hash.
//
// Every variable-length item is written as an 8-byte big-endian length followed
// by its bytes. Because the reader of this stream (there is none — we only
// hash) could unambiguously re-split it, two distinct logical records cannot
// produce the same byte stream regardless of what bytes their values contain.
// This is the standard length-prefix defense against delimiter-injection
// collisions (cf. RFC 8785, Git's object format).
type writer struct {
	h hash.Hash
}

func newWriter() *writer { return &writer{h: sha256.New()} }

// tag writes a single discriminator byte (entity vs. relation) so the two kinds
// share no preimages even when their fields coincide.
func (w *writer) tag(b byte) { w.h.Write([]byte{b}) }

// lenPrefixed writes len(p) as a fixed-width big-endian uint64, then p.
func (w *writer) lenPrefixed(p []byte) {
	var n [8]byte
	binary.BigEndian.PutUint64(n[:], uint64(len(p)))
	w.h.Write(n[:])
	w.h.Write(p)
}

func (w *writer) str(s string) { w.lenPrefixed([]byte(s)) }

// field writes a named string field as two length-prefixed chunks.
func (w *writer) field(key, value string) {
	w.str(key)
	w.str(value)
}

// body writes the markdown body after normalizing it to a formatting fixed
// point, so fsstore's reflowed body and pgstore's raw content converge.
func (w *writer) body(content string) {
	w.str("body")
	w.str(canonicalBody(content))
}

// canonicalBody reduces a markdown body to a formatting fixed point.
//
// fsstore stores FormatMarkdown(raw); pgstore stores raw. Re-running
// FormatMarkdown on read is supposed to converge them, but FormatMarkdown is
// not idempotent for every input (e.g. "0) \n\n0" formats to "\n0\n" then to
// "0\n"), so a single pass can leave fs one step ahead of pg. Iterating to a
// fixed point makes the result independent of how many passes either side has
// already applied. Convergence is fast (≤2 steps observed); the bound is a
// safety net against a pathological non-converging input — if hit, the
// last value is still deterministic for a given input.
func canonicalBody(content string) string {
	const maxPasses = 8
	s := markdown.FormatMarkdown(content)
	for range maxPasses {
		next := markdown.FormatMarkdown(s)
		if next == s {
			return s
		}
		s = next
	}
	return s
}

// properties writes every property in sorted-key order. The count is written
// first (also length-framing the section) and each value is encoded by
// writeValue after [normalize]. A nil or empty map writes a zero count, so an
// entity with no properties and one with an empty (non-nil) map hash
// identically.
func (w *writer) properties(props map[string]any) {
	w.str("props")
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	w.count(len(keys))
	for _, k := range keys {
		w.str(k)
		w.writeValue(normalize(props[k]))
	}
}

// count writes a fixed-width element count, framing variable-length sequences
// (property sets, slices, maps) so their boundaries are unambiguous. n is always
// a len() result and therefore non-negative.
func (w *writer) count(n int) {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(n)) //nolint:gosec // n is a len(), never negative
	w.h.Write(b[:])
}

// writeValue encodes a NORMALIZED value. After [normalize], the only kinds that
// reach here are: nil, string, bool, int64, float64, []any, map[string]any.
// Each is prefixed with a single kind byte so distinct kinds (e.g. the string
// "1" and the int 1) never share a preimage. Any other kind is a normalization
// bug and panics rather than silently risking a cross-backend divergence.
func (w *writer) writeValue(v any) {
	switch val := v.(type) {
	case nil:
		w.h.Write([]byte{'n'})
	case string:
		w.h.Write([]byte{'s'})
		w.str(val)
	case bool:
		w.h.Write([]byte{'b'})
		if val {
			w.h.Write([]byte{1})
		} else {
			w.h.Write([]byte{0})
		}
	case int64:
		w.h.Write([]byte{'i'})
		w.str(strconv.FormatInt(val, 10))
	case float64:
		w.h.Write([]byte{'f'})
		w.str(strconv.FormatFloat(val, 'g', -1, 64))
	case []any:
		w.h.Write([]byte{'a'})
		w.count(len(val))
		for _, item := range val {
			w.writeValue(item)
		}
	case map[string]any:
		w.h.Write([]byte{'m'})
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		w.count(len(keys))
		for _, k := range keys {
			w.str(k)
			w.writeValue(val[k])
		}
	default:
		// normalize is responsible for collapsing every type both backends can
		// produce into the cases above. Reaching here means a new decoder type
		// slipped through — fail loudly rather than hash a value whose
		// cross-backend stability is unverified.
		panic(fmt.Sprintf("canonical: unnormalized value of type %T", v))
	}
}

func (w *writer) sum() string {
	return hex.EncodeToString(w.h.Sum(nil))
}

// normalize folds a property value, whatever concrete Go type a backend's
// decoder produced, into one of the canonical kinds writeValue understands:
// nil, string, bool, int64, float64, []any, map[string]any.
//
// The folds exist to erase the fs-vs-pg representation differences proved by
// the cross-backend tests:
//
//   - All signed/unsigned integer widths -> int64. A uint64 above math.MaxInt64
//     cannot fit int64 and is folded to float64 — the same lossy value pgstore
//     is forced to read back from JSONB, so the two agree (lossily, but
//     identically; see the package doc).
//   - Whole-valued float32/float64 -> int64, matching pgstore's
//     normalizeJSONNumbers fold of "2.0" to 2. Fractional floats stay float64.
//   - time.Time -> its RFC3339 (UTC) string. fsstore decodes a date to
//     time.Time; pgstore round-trips it to exactly this string. Emitting it as
//     a plain string also makes a date and a user-typed identical string hash
//     alike, which is correct: once stored in pg they are indistinguishable.
//   - map[any]any (yaml's form for non-string-keyed mappings) -> map[string]any
//     with stringified keys, recursing into values.
//   - []string -> []any, recursing.
func normalize(v any) any {
	switch val := v.(type) {
	case nil, string, bool:
		return val
	case int:
		return normalizeInt(int64(val))
	case int8:
		return int64(val)
	case int16:
		return int64(val)
	case int32:
		return int64(val)
	case int64:
		return normalizeInt(val)
	case uint:
		return normalizeUint(uint64(val))
	case uint8:
		return int64(val)
	case uint16:
		return int64(val)
	case uint32:
		return int64(val)
	case uint64:
		return normalizeUint(val)
	case float32:
		return normalizeFloat(float64(val))
	case float64:
		return normalizeFloat(val)
	case json.Number:
		// A raw JSON number (json.Decoder.UseNumber), folded the same way
		// pgstore's normalizeJSONNumbers does: whole -> int64, else float64.
		// Handling it here makes canonical self-sufficient even if it ever sees
		// a not-yet-normalized pg value.
		if i, err := val.Int64(); err == nil {
			return normalizeInt(i)
		}
		if f, err := val.Float64(); err == nil {
			return normalizeFloat(f)
		}
		return val.String()
	case time.Time:
		return val.UTC().Format(time.RFC3339Nano)
	case []any:
		out := make([]any, len(val))
		for i, item := range val {
			out[i] = normalize(item)
		}
		return out
	case []string:
		out := make([]any, len(val))
		for i, item := range val {
			out[i] = item
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, item := range val {
			out[k] = normalize(item)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(val))
		for k, item := range val {
			out[stringifyKey(k)] = normalize(item)
		}
		return out
	default:
		// An unrecognized type. Fold via its string form so the hash is at
		// least deterministic; if the two backends ever produce this type they
		// must produce the same string. New types should be added above with a
		// considered fold rather than relying on this fallback.
		return fmt.Sprintf("%v", v)
	}
}

// maxExactInt is 2^53, above which a float64 can no longer represent every
// integer exactly. Whole numbers are only safe to canonicalize as int64 below
// this bound; at or above it the two backends can disagree on the integer (one
// path goes through a lossy float64), so both must canonicalize to the SAME
// lossy float64 to stay equal. See the cross-backend leading-zeros regression.
const maxExactInt = 1 << 53

// normalizeInt folds a signed integer to int64 when it is exactly representable
// as a float64, else to that lossy float64 — because the other backend may have
// routed the same value through a float64 (e.g. yaml decoding a long numeric
// literal) and only the shared lossy form agrees.
func normalizeInt(i int64) any {
	if i >= -maxExactInt && i <= maxExactInt {
		return i
	}
	return float64(i)
}

// normalizeUint folds an unsigned value through the same exact-integer rule.
func normalizeUint(u uint64) any {
	if u <= maxExactInt {
		return int64(u)
	}
	return float64(u)
}

// normalizeFloat folds a whole-valued, exactly-representable float to int64
// (matching pgstore's "2.0" -> 2 normalization) and leaves everything else
// (fractional, non-finite, or beyond the exact-integer range) as float64. The
// exact-range guard is essential: int64(7e17-ish) round-trips lossily, so a
// large whole float must stay float64 to match the other backend's lossy read.
func normalizeFloat(f float64) any {
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return f
	}
	if f == math.Trunc(f) && f >= -maxExactInt && f <= maxExactInt {
		return int64(f)
	}
	return f
}

// stringifyKey renders a non-string map key deterministically. yaml only
// produces scalar keys (string/int/bool/float), so %v is stable here.
func stringifyKey(k any) string {
	if s, ok := k.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", k)
}
