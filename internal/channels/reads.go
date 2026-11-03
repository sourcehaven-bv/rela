package channels

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// The bounds of read markers.
const (
	// UnreadCap is the most unread messages a [ReadState] counts. A reader
	// with more sees the cap, which the SPA shows as "99+".
	UnreadCap = 100
	// MaxReadKeys is the most keys one [Store.Reads] call may name.
	MaxReadKeys = 2000
	// MaxReadWindow is the most markers one [Store.ListReads] call
	// returns.
	MaxReadWindow = 500
)

// Reader is whose read marker a row is: a principal's user and
// organization, as [Author] records them. The tool is not part of it, so a
// user's browser and agent share one marker, and "own post" means what an
// ownership check on the author means.
type Reader struct {
	User  string
	OrgID string
}

// ReaderOf is the reader of the author a.
func ReaderOf(a Author) Reader { return Reader{User: a.User, OrgID: a.OrgID} }

// Validate refuses a reader with no user, a field over [MaxTextBytes], a
// field that is not UTF-8, or a field holding NUL, each [ErrInvalidMessage].
// The length bound is the author bound, so any author the store accepted
// is a valid reader.
func (r Reader) Validate() error {
	if strings.TrimSpace(r.User) == "" {
		return fmt.Errorf("%w: no reader", ErrInvalidMessage)
	}
	for _, v := range []string{r.User, r.OrgID} {
		if len(v) > MaxTextBytes || !utf8.ValidString(v) || strings.ContainsRune(v, 0) {
			return fmt.Errorf("%w: reader field over %d bytes, not UTF-8, or holding NUL",
				ErrInvalidMessage, MaxTextBytes)
		}
	}
	return nil
}

// ReadMarker is a reader's marker on one live channel, as
// [Store.ListReads] returns it.
type ReadMarker struct {
	// Channel is the channel as stored, for the caller's gate.
	Channel Channel
	// Seq is the highest seq read; 0 is "opened, nothing read".
	Seq int64
	// At is when the marker last moved.
	At time.Time
	// LastAt is the later of the channel's newest stored message and its
	// latest lifecycle time (see [LastActivity]). It orders the window.
	LastAt time.Time
}

// ReadState is a marker and what lies above it.
type ReadState struct {
	Seq     int64
	At      time.Time
	LastSeq int64
	// Unread is the number of counting messages above Seq (see [Counts]),
	// at most [UnreadCap].
	Unread int
}

// IsSessionNotice reports whether m is a session notice: a message the
// store wrote for a session move, whose TriggeredBy starts with
// [SessionMarker]. Only the store writes such a message.
func IsSessionNotice(m Message) bool { return strings.HasPrefix(m.TriggeredBy, SessionMarker) }

// Counts reports whether m counts toward a reader's unread: a message that
// is not removed, is not a session notice and was not posted under a type
// with `unread: false` ([Message.NoUnread]). It is the one definition; the
// file backend calls it, and the database backends store its result.
func Counts(m Message) bool { return m.Removed == nil && !IsSessionNotice(m) && !m.NoUnread }

// CheckReads validates the arguments of [Store.Reads]: a valid reader
// ([ErrInvalidMessage]), at most [MaxReadKeys] keys and each key valid
// ([ErrInvalidKey]).
func CheckReads(r Reader, keys []Key) error {
	if err := r.Validate(); err != nil {
		return err
	}
	if len(keys) > MaxReadKeys {
		return fmt.Errorf("%w: %d keys, over %d", ErrInvalidKey, len(keys), MaxReadKeys)
	}
	for _, k := range keys {
		if err := k.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// CheckSetRead validates the arguments of [Store.SetRead]: a valid key
// ([ErrInvalidKey]), a valid reader and a seq of 0 or more
// ([ErrInvalidMessage]).
func CheckSetRead(k Key, r Reader, seq int64) error {
	if err := k.Validate(); err != nil {
		return err
	}
	if err := r.Validate(); err != nil {
		return err
	}
	if seq < 0 {
		return fmt.Errorf("%w: negative read seq %d", ErrInvalidMessage, seq)
	}
	return nil
}

// CheckListReads validates the arguments of [Store.ListReads]: a valid
// reader and a limit from 1 to [MaxReadWindow], each failure
// [ErrInvalidMessage].
func CheckListReads(r Reader, limit int) error {
	if err := r.Validate(); err != nil {
		return err
	}
	if limit < 1 || limit > MaxReadWindow {
		return fmt.Errorf("%w: read window %d is not from 1 to %d", ErrInvalidMessage, limit, MaxReadWindow)
	}
	return nil
}

// ClampRead is the seq a [Store.SetRead] stores: the asked seq, at most
// the channel's last seq, and never below the old marker.
func ClampRead(old, asked, lastSeq int64) int64 { return max(old, min(asked, lastSeq)) }

// LifecycleAt is the latest of the channel's created, closed, archived and
// reopened times: its last activity when it holds no message since.
func LifecycleAt(c Channel) time.Time {
	at := c.CreatedAt
	for _, t := range []time.Time{c.ClosedAt, c.ArchivedAt, c.ReopenedAt} {
		if t.After(at) {
			at = t
		}
	}
	return at
}

// LastActivity is the LastAt of a [ReadMarker] on c: the later of msgAt,
// the time of the newest message c stored (zero when none; a removed
// message keeps its time), and [LifecycleAt]. Every backend computes it
// this way, so the window orders alike.
func LastActivity(c Channel, msgAt time.Time) time.Time {
	at := LifecycleAt(c)
	if msgAt.After(at) {
		at = msgAt
	}
	return NormTime(at)
}

// SortReadWindow sorts ms into window order and cuts it to limit: markers
// on channels with messages above the marker first, then the rest; within
// each group the newest LastAt first, then [KeyLess]. A file backend that
// sorts in Go uses it, so the backends agree on the window. The first group
// is decided by LastSeq, not by the count, so a channel with only
// non-counting messages above the marker (session notices, or types with
// `unread: false`) sorts there with an unread count of 0.
func SortReadWindow(ms []ReadMarker, limit int) []ReadMarker {
	sort.Slice(ms, func(i, j int) bool {
		a, b := ms[i], ms[j]
		au, bu := a.Channel.LastSeq > a.Seq, b.Channel.LastSeq > b.Seq
		if au != bu {
			return au
		}
		if !a.LastAt.Equal(b.LastAt) {
			return a.LastAt.After(b.LastAt)
		}
		return KeyLess(a.Channel.Key, b.Channel.Key)
	})
	if len(ms) > limit {
		ms = ms[:limit]
	}
	return ms
}
