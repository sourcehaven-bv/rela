package records

import (
	"cmp"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Cursor encodes the position after rec for [ListQuery.After].
func Cursor(rec Record) string {
	return strconv.FormatInt(rec.TS.UnixMicro(), 10) + ":" + rec.ID
}

// ParseCursor decodes a cursor into the TS (in microseconds) and ID it
// points after. Empty is the start.
func ParseCursor(c string) (micros int64, id string, err error) {
	if c == "" {
		return 0, "", nil
	}
	ts, rest, ok := strings.Cut(c, ":")
	if !ok || rest == "" {
		return 0, "", fmt.Errorf("%w: bad cursor", ErrInvalid)
	}
	micros, err = strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return 0, "", fmt.Errorf("%w: bad cursor", ErrInvalid)
	}
	return micros, rest, nil
}

// After reports whether rec sorts after the cursor position (micros, id).
func After(rec Record, micros int64, id string) bool {
	if c := cmp.Compare(rec.TS.UnixMicro(), micros); c != 0 {
		return c > 0
	}
	return rec.ID > id
}

// Compare orders rows by (TS, ID), the order List returns.
func Compare(a, b Record) int {
	if c := cmp.Compare(a.TS.UnixMicro(), b.TS.UnixMicro()); c != 0 {
		return c
	}
	return cmp.Compare(a.ID, b.ID)
}

// InRange reports whether ts lies in [from, to); a zero bound is open.
func InRange(ts, from, to time.Time) bool {
	if !from.IsZero() && ts.Before(from) {
		return false
	}
	return to.IsZero() || ts.Before(to)
}

// PageLimit returns the page size q asks for, clamped to [1, MaxPage].
func PageLimit(q ListQuery) int {
	switch {
	case q.Limit <= 0:
		return DefaultPage
	case q.Limit > MaxPage:
		return MaxPage
	}
	return q.Limit
}

// Micros returns t in microseconds since the epoch, the precision every
// backend stores.
func Micros(t time.Time) int64 { return t.UnixMicro() }

// FromMicros is the inverse of [Micros], in UTC.
func FromMicros(v int64) time.Time { return time.UnixMicro(v).UTC() }
