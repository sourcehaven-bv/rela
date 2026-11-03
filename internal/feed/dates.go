package feed

import (
	"fmt"
	"strings"
	"time"
)

// dateLayouts covers RFC 822 / 1123 / 850 dates as RSS uses them (with and
// without weekday, two- or four-digit years) and the RFC 3339 forms Atom and
// JSON Feed use. Every layout with a zone takes a numeric offset: a named
// zone is rewritten to one by [numericZone] first. Layouts without a zone
// read as UTC.
var dateLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05-0700",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02",
	"Mon, 2 Jan 2006 15:04:05 -0700",
	"Mon, 2 Jan 2006 15:04 -0700",
	"Mon, 2 Jan 06 15:04:05 -0700",
	"2 Jan 2006 15:04:05 -0700",
	"2 Jan 06 15:04:05 -0700",
	"Monday, 02-Jan-06 15:04:05 -0700",
}

// namedZones are the zone names RFC 822 defines, in hours from UTC.
// time.Parse would accept any three-letter name and, unless it is the host's
// own zone, give it a zero offset, so a US feed's dates would shift by hours
// depending on where rela runs. Names outside this table are not guessed.
var namedZones = map[string]int{
	"UT": 0, "UTC": 0, "GMT": 0, "Z": 0,
	"EST": -5, "EDT": -4, "CST": -6, "CDT": -5,
	"MST": -7, "MDT": -6, "PST": -8, "PDT": -7,
}

// parseDate returns s as a UTC time, or the zero time when no layout fits
// or the zone is a name it cannot resolve.
func parseDate(s string) time.Time {
	s, ok := numericZone(strings.TrimSpace(s))
	if !ok || s == "" {
		return time.Time{}
	}
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

// numericZone replaces a trailing alphabetic zone name with its numeric
// offset. It reports false for a name not in [namedZones].
func numericZone(s string) (string, bool) {
	i := strings.LastIndexByte(s, ' ')
	if i < 0 {
		return s, true
	}
	zone := s[i+1:]
	if !isAlpha(zone) {
		return s, true
	}
	hours, ok := namedZones[strings.ToUpper(zone)]
	if !ok {
		return "", false
	}
	sign := '+'
	if hours < 0 {
		sign, hours = '-', -hours
	}
	return fmt.Sprintf("%s %c%02d00", s[:i], sign, hours), true
}

func isAlpha(s string) bool {
	for _, r := range s {
		if (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') {
			return false
		}
	}
	return s != ""
}
