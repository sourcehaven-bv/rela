package fsimport

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// normalizeProps converts property values read from YAML frontmatter into the
// form a database backend stores, which is JSON.
//
// YAML decodes an unquoted date or timestamp as time.Time, which JSON would
// store as an RFC 3339 timestamp. The conversion keeps what the author wrote:
//
//   - On a `date` property, a time.Time becomes "YYYY-MM-DD". A value with a
//     time of day is an error: dropping the time would change the data.
//   - On a `datetime` property, it becomes RFC 3339.
//   - Anywhere else, a bare date (midnight UTC, which is how YAML decodes
//     "2026-03-04") becomes "YYYY-MM-DD" and a timestamp becomes RFC 3339,
//     so a string property keeps the text the author wrote.
//
// Every other value is stored as is, provided JSON can hold it: NaN,
// infinities and maps with non-string keys cannot, and are errors rather
// than silent changes.
//
// changed counts the converted values, for the report.
func normalizeProps(
	props map[string]any, defs map[string]metamodel.PropertyDef,
) (out map[string]any, changed int, err error) {
	if len(props) == 0 {
		return props, 0, nil
	}
	out = make(map[string]any, len(props))
	for _, name := range slices.Sorted(maps.Keys(props)) {
		v, n, verr := normalizeValue(props[name], defs[name].Type)
		if verr != nil {
			return nil, 0, fmt.Errorf("property %q: %w", name, verr)
		}
		if _, jerr := json.Marshal(v); jerr != nil {
			return nil, 0, fmt.Errorf("property %q holds a value a database cannot store: %w", name, jerr)
		}
		out[name] = v
		changed += n
	}
	return out, changed, nil
}

// normalizeValue converts every time.Time inside v, for a property of type
// typ. Lists and maps are walked because a list property of dates decodes as
// []any of time.Time. changed counts the values converted.
func normalizeValue(v any, typ string) (out any, changed int, err error) {
	switch val := v.(type) {
	case time.Time:
		s, ferr := formatTime(val, typ)
		return s, 1, ferr
	case []any:
		list := make([]any, len(val))
		for i, item := range val {
			var n int
			if list[i], n, err = normalizeValue(item, typ); err != nil {
				return nil, 0, err
			}
			changed += n
		}
		return list, changed, nil
	case map[string]any:
		m := make(map[string]any, len(val))
		for k, item := range val {
			var n int
			if m[k], n, err = normalizeValue(item, typ); err != nil {
				return nil, 0, err
			}
			changed += n
		}
		return m, changed, nil
	default:
		return v, 0, nil
	}
}

func formatTime(t time.Time, typ string) (string, error) {
	midnight := t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 && t.Nanosecond() == 0
	switch {
	case typ == metamodel.PropertyTypeDate && midnight:
		return t.Format(time.DateOnly), nil
	case typ == metamodel.PropertyTypeDate:
		return "", fmt.Errorf("date property holds a time of day (%s); write the date alone",
			t.Format(time.RFC3339))
	case typ != metamodel.PropertyTypeDatetime && midnight && t.Location() == time.UTC:
		return t.Format(time.DateOnly), nil
	default:
		return t.Format(time.RFC3339Nano), nil
	}
}
