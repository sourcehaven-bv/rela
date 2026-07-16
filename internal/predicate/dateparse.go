package predicate

import (
	"fmt"
	"time"
)

// Built-in fallback date layouts, used when a DateType carries no
// explicit layout. They mirror internal/metamodel's DefaultDateFormat
// ("2006-01-02") and DefaultDatetimeFormat (RFC3339) so a predicate
// authored against an un-adapted env still parses the two canonical
// forms. The metamodel->Env adapter should supply the field's real
// layout via DateTypeWithLayout so a custom format is honored.
var defaultDateLayouts = []string{"2006-01-02", time.RFC3339}

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
