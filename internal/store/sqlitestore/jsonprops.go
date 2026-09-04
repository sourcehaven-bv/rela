package sqlitestore

import (
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"

	"bytes"
	"encoding/json"
	"fmt"
)

// Property maps are stored as a JSON document in a TEXT column, so every value
// round-trips through encoding/json — and that is where a SQL backend quietly
// diverges from the in-memory ones.
//
// Plain json.Unmarshal decodes EVERY number to float64, so an entity written
// with `count: 5` reads back as `float64(5)`. memstore and fsstore return
// `int(5)`. The conformance suite catches it (Relation/CreateWithData), but
// only because it asserts on a numeric property — it is otherwise the kind of
// difference that surfaces much later as a formatting or comparison bug.
//
// The same normalization exists in pgstore for the same reason, and
// internal/canonical documents that it folds numbers "the same way pgstore's
// normalizeJSONNumbers does". This is therefore the third copy. It is left
// local rather than hoisted because hoisting it touches pgstore's write path,
// which is a separate change with its own risk — TKT-L3FNEN's neighborhood,
// filed rather than smuggled in here.

// marshalProps encodes a property map for storage. An empty map is stored as
// "{}" rather than NULL so every row has a valid JSON document and json_extract
// never has to special-case a missing column.
func marshalProps(p map[string]any) (string, error) {
	// The one place properties become JSON here (entity and relation rows);
	// see pgstore's marshalProps for why the text gate sits at the
	// serializer rather than at each caller (BUG-X7ICNM).
	if err := storeutil.ValidateProperties(p); err != nil {
		return "", err
	}
	if len(p) == 0 {
		return "{}", nil
	}
	b, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("sqlitestore: marshal properties: %w", err)
	}
	return string(b), nil
}

// unmarshalProps decodes a stored property map, normalizing numbers so whole
// values come back as int rather than float64.
//
// Nil: returns a nil map for an empty or "{}" document, matching what a store
// returns for an entity with no properties. That is a valid result, not a
// missing one — an entity legitimately has no properties.
//
//nolint:nilnil // a nil map with no error IS the "no properties" answer here
func unmarshalProps(raw string) (map[string]any, error) {
	if raw == "" || raw == "{}" {
		return nil, nil
	}
	dec := json.NewDecoder(bytes.NewReader([]byte(raw)))
	// UseNumber defers the int-vs-float decision to normalizeJSONNumbers
	// instead of letting the decoder collapse everything to float64.
	dec.UseNumber()

	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("sqlitestore: unmarshal properties: %w", err)
	}
	return normalizeJSONMap(m), nil
}

// normalizeJSONNumbers walks a decoded value and converts json.Number to int
// when it has no fractional part, else float64. Strings, bools and nested
// maps/slices are preserved structurally.
func normalizeJSONNumbers(v any) any {
	switch t := v.(type) {
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return int(i)
		}
		if f, err := t.Float64(); err == nil {
			return f
		}
		// Neither representation fits (an integer beyond int64, say). Keeping
		// the literal text loses less than silently truncating it.
		return t.String()
	case map[string]any:
		return normalizeJSONMap(t)
	case []any:
		for i := range t {
			t[i] = normalizeJSONNumbers(t[i])
		}
		return t
	default:
		return v
	}
}

func normalizeJSONMap(m map[string]any) map[string]any {
	for k, v := range m {
		m[k] = normalizeJSONNumbers(v)
	}
	return m
}
