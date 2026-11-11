package remotesync

import (
	"time"
)

// The reasons a conflict under the latest rule is held instead of decided.
// They are written to [Conflict.Fallback].
const (
	// FallbackTimeWithheld: a side did not serve its last-updated time. The
	// remote withholds it when a field of the row is hidden from this
	// instance; locally it is withheld the same way from the run's
	// principal, and a backend may keep no time at all.
	FallbackTimeWithheld = "time_withheld"
	// FallbackTimeUntrusted: a time lies beyond now plus max_clock_skew,
	// before the last sync of the row, or does not parse.
	FallbackTimeUntrusted = "time_untrusted"
	// FallbackSameTime: both sides changed the row at the same time.
	FallbackSameTime = "same_time"
)

// stamp is one side's last-updated time of a row, as the run may use it.
// A zero At that is not Untrusted means the side served no time.
type stamp struct {
	At time.Time
	// Untrusted is set when the side served a time that does not parse.
	Untrusted bool
}

// parseStamp reads a remote `_updated` value. An empty value is a withheld
// time.
func parseStamp(raw string) stamp {
	if raw == "" {
		return stamp{}
	}
	t, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return stamp{Untrusted: true}
	}
	return stamp{At: t}
}

// verdict is the outcome of the latest rule for one row: the side that
// changed it last, or the reason it is held instead.
type verdict struct {
	winner   string
	fallback string
	local    time.Time
	remote   time.Time
}

// latest judges which side changed a row last. It is computed per row,
// because the API serves one time per row, not per field: an edit to
// another field of the row moves the time too.
//
// A time is trusted when it is set, parses, lies no further in the future
// than skew, and lies no earlier than synced minus skew: a side that changed
// a field since the last sync cannot have done so before it. Equal times
// decide nothing.
func latest(local, remote stamp, synced, now time.Time, skew time.Duration) verdict {
	v := verdict{local: local.At, remote: remote.At}
	switch {
	case local.Untrusted || remote.Untrusted:
		v.fallback = FallbackTimeUntrusted
	case local.At.IsZero() || remote.At.IsZero():
		v.fallback = FallbackTimeWithheld
	case !trusted(local.At, synced, now, skew) || !trusted(remote.At, synced, now, skew):
		v.fallback = FallbackTimeUntrusted
	case local.At.Equal(remote.At):
		v.fallback = FallbackSameTime
	case local.At.After(remote.At):
		v.winner = TakeLocal
	default:
		v.winner = TakeRemote
	}
	return v
}

func trusted(t, synced, now time.Time, skew time.Duration) bool {
	if t.After(now.Add(skew)) {
		return false
	}
	return synced.IsZero() || !t.Before(synced.Add(-skew))
}

// decide applies a field's rule to a conflict. It returns the side whose
// value stands, or "" with the reason when the field is held.
func decide(rule string, v verdict) (winner, fallback string) {
	switch rule {
	case ConflictLocal:
		return TakeLocal, ""
	case ConflictRemote:
		return TakeRemote, ""
	case ConflictLatest:
		return v.winner, v.fallback
	}
	return "", ""
}
