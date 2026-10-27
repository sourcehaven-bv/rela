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
	// MaxSessionStates caps the states one channel type's session declares.
	MaxSessionStates = 64
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
	// EphemeralRingShare caps the live signals of one identity in one
	// channel's ring. It is well below half of [EphemeralRingSize], so no
	// two identities can fill a ring between them. A signal over the share
	// replaces the poster's own oldest signal in that ring.
	EphemeralRingShare = 8
	// MaxEphemeralChannelsPerIdentity caps the channels in which one
	// identity holds live signals in one process. With
	// [MaxEphemeralChannels], it takes 1024 identities to occupy every
	// channel slot.
	MaxEphemeralChannelsPerIdentity = 4
	// MaxEphemeralChannels caps the channels that hold live signals in one
	// process. A signal to another channel is refused while the cap holds.
	MaxEphemeralChannels = 4096
	// MaxRateBucketsPerIdentity caps the per-channel rate-limit buckets of
	// one identity in one process. A request that needs another one is
	// refused for that identity only.
	MaxRateBucketsPerIdentity = 64
	// MaxRateBuckets caps the per-channel rate-limit buckets of one process,
	// across identities. At [MaxRateBucketsPerIdentity] each, it takes 512
	// identities writing at once to fill it. A request that needs a new
	// bucket is refused while the cap holds.
	MaxRateBuckets = 32768
	// MaxRateIdentities caps the identities one process keeps rate-limit
	// state for. Their aggregate buckets live apart from the per-channel
	// ones, so filling [MaxRateBuckets] never keeps a new identity out.
	MaxRateIdentities = 16384
	// MaxSubscriberChanges caps the changed channels one live subscriber
	// accumulates between two takes. Past it, the subscriber is told to
	// resync everything instead.
	MaxSubscriberChanges = 1024
)

// Typing indicator. The server owns these numbers: `limits:` in
// realtime.yaml does not name them and cannot change them.
const (
	// TypingTTL is how long a typing entry stays live after its last
	// refresh. The expiry timer drops it then and wakes readers.
	TypingTTL = 6 * time.Second
	// MaxTypingPerChannel caps the typing entries of one channel. A new
	// typist over the cap is refused; no entry is evicted.
	MaxTypingPerChannel = 32
	// MaxTypingChannels caps the channels that hold typing entries in one
	// process. A typist in another channel is refused while the cap holds.
	MaxTypingChannels = 4096
)

// Retention defaults and waits of the background sweep.
const (
	// DetachedRetention is how long a channel detached from its entity is
	// kept before the sweep erases it, unless the channel type sets
	// `detached_retention:`. It covers the detach and block policies, an
	// orphaned channel, and a channel of a type the config no longer
	// declares.
	DetachedRetention = 30 * 24 * time.Hour
	// OrphanGrace is how long a channel's entity must be missing, or a
	// detached batch must stay unconfirmed, before the sweep acts on it.
	OrphanGrace = time.Hour
	// PendingRelease is how long an action must stay pending before the
	// sweep reports it and a moderator may release it.
	PendingRelease = 15 * time.Minute
)

// Pace and caps of the trigger engine.
const (
	// TriggerTick is how often the engine looks at the entities that
	// changed. Changes inside one tick are seen together, so an entity
	// that starts and stops matching between two ticks posts nothing.
	TriggerTick = 500 * time.Millisecond
	// TriggerReconcile is how often the engine compares every entity with
	// what it has seen. It catches changes that raised no event, such as a
	// rule on today() that starts to hold or an event the store dropped.
	TriggerReconcile = 15 * time.Minute
	// MaxTriggerPending caps the changed entities the engine holds between
	// ticks. Past it, the changes of a type are dropped and the type is
	// compared in full at the next tick instead.
	MaxTriggerPending = 8192
	// MaxSystemPostsPerRound caps the posts the engine attempts in one
	// tick, shared by its rules in turn. Every attempt counts, whether it
	// posts, finds the channel closed or fails. The rest wait in their
	// rule's queue for later ticks.
	MaxSystemPostsPerRound = 64
	// MaxTriggerQueue caps the posts waiting in the rules' queues. Past it
	// a post is not queued and its change is not counted as seen, so the
	// next reconcile finds it again.
	MaxTriggerQueue = 65536
	// TriggerCooldown is the least time between two posts of one trigger
	// about one entity. Changes in between are seen together when it ends.
	TriggerCooldown = time.Minute
	// TriggerRetryWindow is how long the engine retries a post that fails
	// for a reason that may pass, such as a database failover. Then it
	// logs the failure and counts the change as seen, so it never repeats a
	// post forever. A post that can never succeed, such as one whose props
	// no longer fit the message type, is given up at once.
	TriggerRetryWindow = 5 * time.Minute
	// TriggerRetryMaxDelay caps the wait between two retries, which
	// doubles from [TriggerTick].
	TriggerRetryMaxDelay = time.Minute
	// SeenPerShard is how many seen entities one stored shard of a rule
	// holds before the rule is split over more shards.
	SeenPerShard = 2048
	// MaxSeenShards caps the shards of one rule.
	MaxSeenShards = 256
)

// Session timers (`after:` on a session state). The server owns these
// numbers: `limits:` in realtime.yaml does not name them.
const (
	// MaxSessionTimers caps the timers of one session state.
	MaxSessionTimers = 8
	// MinSessionTimer is the shortest fixed period a timer may wait. A
	// calendar period (months, years) is always longer.
	MinSessionTimer = time.Minute
	// TimerFloor is the least time between two timer rounds.
	TimerFloor = time.Second
	// TimerCeiling is the most time between two timer rounds. A move made
	// in another process reaches the leader within it.
	TimerCeiling = time.Minute
	// TimerRecompute is how often the leader checks the stored due time of
	// every open session against the configuration.
	TimerRecompute = 10 * time.Minute
	// MaxTimerWritesPerRound caps the fires and reschedules of one round.
	// The rest wait for the next round, one floor later.
	MaxTimerWritesPerRound = 64
	// MaxTimerListPerRound caps the due sessions one round lists.
	MaxTimerListPerRound = 256
	// TimerBackoff is how long a round skips a channel whose write failed.
	TimerBackoff = 5 * time.Minute
	// MaxTimerBackoffKeys caps the channels held in backoff; the oldest is
	// dropped past it.
	MaxTimerBackoffKeys = 128
)

// Session reminders (`session.reminders`). Their every and first reuse
// [MinSessionTimer], and their posts share [MaxTimerWritesPerRound].
const (
	// MaxSessionReminders caps the reminders of one session.
	MaxSessionReminders = 8
	// MaxReminderCount is the largest `max` a reminder may declare.
	MaxReminderCount = 1000
	// ReminderSlack is added to a reminder's next slot in the stored due
	// time, so one leader round serves several nearby slots.
	ReminderSlack = 30 * time.Second
)

// Session outputs (`session.states.<s>.on.<type>.outputs`) and the runner
// that retries them.
const (
	// MaxTransitionOutputs caps the outputs of one transition.
	MaxTransitionOutputs = 4
	// MaxOutputTemplateBytes caps one output property template.
	MaxOutputTemplateBytes = 1024
	// OutputClaimTTL is how long a claim on an outputs mark holds after
	// its holder's last write. It is more than twice one output step (the
	// promote timeout plus a progress write), so a live holder is never
	// taken over.
	OutputClaimTTL = 90 * time.Second
	// MaxOutputAttempts caps the attempts at one outputs mark.
	MaxOutputAttempts = 5
	// MaxOutputRunsPerRound caps the marks one runner round retries.
	MaxOutputRunsPerRound = 16
	// OutputRetryWindow is how long after its notice a mark may still be
	// retried. The retry rebuilds the poster from stored role claims, and
	// the window bounds how long those claims count.
	OutputRetryWindow = 30 * time.Minute
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
	// edit, remove and the lifecycle operations.
	OpRate = Rate{Count: 30, Per: time.Minute}
	// TypingRate applies to typing per identity and channel. The server
	// owns it; `limits.signal_rate` does not apply to typing.
	TypingRate = Rate{Count: 1, Per: time.Second}
	// ReadRate applies to marking a channel read, per reader and channel:
	// one a second on average, with a burst of two.
	ReadRate = Rate{Count: 2, Per: 2 * time.Second}
	// ReadIdentityRate caps every mark of one reader together. Marks have
	// their own limiter, so they never use up the post budget of
	// IdentityRate.
	ReadIdentityRate = Rate{Count: 300, Per: time.Minute}
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
