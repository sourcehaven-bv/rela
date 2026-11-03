package realtimecfg

import (
	"errors"
	"strconv"
	"strings"
	"time"
)

// Period is a span of time written as a count and a unit: 30s, 15m, 2h, 7d,
// 3mo or 7y. Seconds, minutes, hours and days have a fixed length. Months and
// years are calendar units, so adding one month to 31 January gives 3 March
// (Go's [time.Time.AddDate] arithmetic). The zero Period is "not set".
type Period struct {
	fixed  time.Duration
	months int
	text   string
}

// maxPeriodCount bounds the count so that count times the longest fixed
// unit (a day) fits in a [time.Duration]. Without it a large retention
// would overflow to a negative duration, which means "delete now".
const maxPeriodCount = 100000

var errPeriodSyntax = errors.New(`must be a count from 1 to 100000 and a unit: s, m, h, d, mo or y (for example "15m" or "7y")`)

// ParsePeriod reads a period such as "15m", "30d" or "7y".
func ParsePeriod(s string) (Period, error) {
	s = strings.TrimSpace(s)
	i := strings.IndexFunc(s, func(r rune) bool { return r < '0' || r > '9' })
	if i <= 0 {
		return Period{}, errPeriodSyntax
	}
	n, err := strconv.Atoi(s[:i])
	if err != nil || n <= 0 || n > maxPeriodCount {
		return Period{}, errPeriodSyntax
	}
	p := Period{text: s}
	switch s[i:] {
	case "s":
		p.fixed = time.Duration(n) * time.Second
	case "m":
		p.fixed = time.Duration(n) * time.Minute
	case "h":
		p.fixed = time.Duration(n) * time.Hour
	case "d":
		p.fixed = time.Duration(n) * 24 * time.Hour
	case "mo":
		p.months = n
	case "y":
		p.months = 12 * n
	default:
		return Period{}, errPeriodSyntax
	}
	return p, nil
}

// IsZero reports whether the period is unset.
func (p Period) IsZero() bool { return p.fixed == 0 && p.months == 0 }

// Fixed returns the length of a period in s, m, h or d, and false for a
// calendar period, whose length depends on the date it starts from.
func (p Period) Fixed() (time.Duration, bool) { return p.fixed, p.months == 0 }

// AddTo returns t plus the period.
func (p Period) AddTo(t time.Time) time.Time {
	if p.months != 0 {
		return t.AddDate(0, p.months, 0)
	}
	return t.Add(p.fixed)
}

// calendarSlack covers the days [time.Time.AddDate] can add past a month
// end: 31 January plus one month is 3 March.
const calendarSlack = 4 * 24 * time.Hour

// Cutoff returns a time c such that every start s with p.AddTo(s) <= now
// lies before c. For a fixed period it is tight; for a calendar period it
// is a few days late, so a caller lists the candidates before c and then
// checks each one exactly with AddTo.
func (p Period) Cutoff(now time.Time) time.Time {
	if p.months != 0 {
		return now.AddDate(0, -p.months, 0).Add(calendarSlack)
	}
	return now.Add(-p.fixed).Add(time.Microsecond)
}

// String returns the period as written.
func (p Period) String() string { return p.text }
