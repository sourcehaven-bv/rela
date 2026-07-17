package predicate

import (
	"fmt"
	"time"
)

// Built-in fallback date layouts. They mirror the fallback set
// internal/metamodel.ParseDateValue accepts (validation.go) so a date
// literal that internal/filter's --where would accept also coerces here
// — keeping predicate a true superset of filter (RR-BNRMU). The
// metamodel->Env adapter supplies the field's declared layout via
// DateTypeWithLayout; these are only the fallbacks tried after it.
var defaultDateLayouts = []string{
	time.RFC3339,           // 2006-01-02T15:04:05Z07:00
	"2006-01-02T15:04:05Z", // ISO 8601 with Z
	"2006-01-02T15:04:05",  // ISO 8601 without timezone
	"2006-01-02",           // ISO 8601 date only
}

// parseDateLiteral parses a date/datetime literal string. If layout is
// non-empty it is tried first (the field's declared format); the
// built-in defaults are always tried as a fallback so an RFC3339
// datetime literal still parses against a bare-date field and vice
// versa. Parsing happens at COMPILE time only — never at Eval — which
// is what keeps the engine's no-I/O-at-eval invariant intact
// (RR-A3EZR).
func parseDateLiteral(s, layout string) (time.Time, error) {
	layouts := make([]string, 0, len(defaultDateLayouts)+1)
	if layout != "" {
		layouts = append(layouts, layout)
	}
	layouts = append(layouts, defaultDateLayouts...)

	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, nil
		}
	}
	shown := layout
	if shown == "" {
		shown = defaultDateLayouts[0]
	}
	return time.Time{}, fmt.Errorf("invalid date literal %q (expected format %q)", s, shown)
}
