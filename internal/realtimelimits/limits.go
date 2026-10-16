// Package realtimelimits holds the built-in caps for realtime channels and
// signals. realtime.yaml may lower them and never raise them, and the channel
// store and service enforce them, so both read the same numbers from here.
//
// It is a leaf: it imports nothing from rela.
package realtimelimits

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Built-in caps. realtime.yaml `limits:` may lower the ones it names.
const (
	// MaxMessageBytes caps the JSON encoding of one message's properties.
	MaxMessageBytes = 16 << 10
	// MaxMessageProperties caps the properties one message type declares.
	MaxMessageProperties = 32
	// MaxChannelsPerAnchor caps the channels of one type on one entity.
	MaxChannelsPerAnchor = 200
	// MaxPageLimit caps the messages one page returns.
	MaxPageLimit = 200
	// EphemeralRingSize is how many signals a channel keeps in memory.
	EphemeralRingSize = 64
	// MaxEphemeralTTL caps how long a signal stays visible.
	MaxEphemeralTTL = 60 * time.Second
	// MaxTitleRunes caps the title of a channel.
	MaxTitleRunes = 200
	// MaxReasonRunes caps the reason given for removing a message.
	MaxReasonRunes = 500
	// MaxEphemeralBytes caps the JSON encoding of one signal's properties.
	// A signal is held in memory, so it is far smaller than a message.
	MaxEphemeralBytes = 1 << 10
	// MaxEphemeralTotalBytes caps the properties of every live signal in
	// one process together. A signal over the budget is refused.
	MaxEphemeralTotalBytes = 8 << 20
	// MaxEphemeralPerIdentity caps the live signals of one identity in one
	// process. A signal over the cap is refused; none is evicted.
	MaxEphemeralPerIdentity = 32
	// MaxEphemeralChannels caps the channels that hold live signals in one
	// process. A signal to another channel is refused while the cap holds.
	MaxEphemeralChannels = 4096
	// MaxRateBuckets caps the rate-limit buckets one process keeps. A
	// request that needs a new bucket is refused while the cap holds.
	MaxRateBuckets = 8192
)

// Rate is a token-bucket rate: Count events per Per.
type Rate struct {
	Count int
	Per   time.Duration
}

// Default rates per identity, channel and message type.
var (
	// DefaultLogRate applies to message types that persist to the log.
	DefaultLogRate = Rate{Count: 30, Per: time.Minute}
	// DefaultEphemeralRate applies to signals (ephemeral message types).
	DefaultEphemeralRate = Rate{Count: 2, Per: time.Second}
	// IdentityRate caps every channel write of one identity together
	// (user and organization), across channels, operations and types.
	IdentityRate = Rate{Count: 300, Per: time.Minute}
	// OpRate applies to the channel operations that are not a post: create,
	// edit and remove.
	OpRate = Rate{Count: 30, Per: time.Minute}
)

// PerSecond returns the rate as events per second.
func (r Rate) PerSecond() float64 {
	if r.Per <= 0 {
		return 0
	}
	return float64(r.Count) / r.Per.Seconds()
}

// AtMost reports whether r allows no more events than limit does, both on
// average and in one burst. A token bucket holds Count tokens, so a rate with
// a lower average but a larger Count still admits a bigger burst: 1000/h
// averages less than 30/m, yet lets 1000 posts through at once.
func (r Rate) AtMost(limit Rate) bool {
	return r.Count <= limit.Count && r.PerSecond() <= limit.PerSecond()
}

// String renders the rate the way [ParseRate] reads it.
func (r Rate) String() string {
	var unit string
	switch r.Per {
	case time.Minute:
		unit = "m"
	case time.Hour:
		unit = "h"
	default:
		unit = "s"
	}
	return fmt.Sprintf("%d/%s", r.Count, unit)
}

var errRateSyntax = errors.New(`must look like "30/m": a positive count, a slash, and s, m or h`)

// ParseRate reads a rate such as "30/m", "2/s" or "100/h".
func ParseRate(s string) (Rate, error) {
	count, unit, ok := strings.Cut(strings.TrimSpace(s), "/")
	if !ok {
		return Rate{}, errRateSyntax
	}
	n, err := strconv.Atoi(count)
	if err != nil || n <= 0 {
		return Rate{}, errRateSyntax
	}
	var per time.Duration
	switch unit {
	case "s":
		per = time.Second
	case "m":
		per = time.Minute
	case "h":
		per = time.Hour
	default:
		return Rate{}, errRateSyntax
	}
	return Rate{Count: n, Per: per}, nil
}
