package feed

import (
	"strings"
	"time"
)

// dateLayouts covers RFC 822 / 1123 dates as RSS uses them (with and without
// weekday, with two- or four-digit years, numeric or named zones) and the
// RFC 3339 forms Atom and JSON Feed use. Layouts without a zone read as UTC.
var dateLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02",
	time.RFC1123Z,
	time.RFC1123,
	"Mon, 2 Jan 2006 15:04:05 -0700",
	"Mon, 2 Jan 2006 15:04:05 MST",
	"Mon, 2 Jan 2006 15:04 -0700",
	"Mon, 2 Jan 2006 15:04 MST",
	"Mon, 2 Jan 06 15:04:05 -0700",
	"Mon, 2 Jan 06 15:04:05 MST",
	"2 Jan 2006 15:04:05 -0700",
	"2 Jan 2006 15:04:05 MST",
	"2 Jan 06 15:04:05 -0700",
	"2 Jan 06 15:04:05 MST",
}

// namedZones are the zone abbreviations RFC 822 defines. time.Parse knows
// only the abbreviations of the local zone and gives any other one a zero
// offset, which would shift a US feed's dates by hours.
var namedZones = map[string]int{
	"UT": 0, "GMT": 0, "Z": 0, "UTC": 0,
	"EST": -5, "EDT": -4, "CST": -6, "CDT": -5,
	"MST": -7, "MDT": -6, "PST": -8, "PDT": -7,
}

// parseDate returns s as a UTC time, or the zero time when no layout fits.
func parseDate(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	for _, layout := range dateLayouts {
		t, err := time.Parse(layout, s)
		if err != nil {
			continue
		}
		if strings.Contains(layout, "MST") {
			t = applyNamedZone(t)
		}
		return t.UTC()
	}
	return time.Time{}
}

// applyNamedZone corrects the offset of a time parsed with a named zone.
func applyNamedZone(t time.Time) time.Time {
	name, _ := t.Zone()
	hours, ok := namedZones[strings.ToUpper(name)]
	if !ok {
		return t
	}
	wall := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC)
	return wall.Add(-time.Duration(hours) * time.Hour)
}
