package channels

import (
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// The bounds of addressed rows (principal mentions and replies).
const (
	// MaxMentions is the most targets one message addresses. The channel
	// service uses this constant too, so the 422 it answers and the store's
	// refusal agree.
	MaxMentions = 10
	// MaxAddressedPerTarget is the most rows one target keeps in one
	// channel. See [Evict] for which row a full set frees.
	MaxAddressedPerTarget = 200
	// MaxAddressedPerFile is the most rows the file backend keeps in one
	// channel's mentions.json, over every target.
	MaxAddressedPerFile = 5000
	// MaxAddressedScan is how many of a target's newest rows of the asked
	// kind [Store.ListAddressed] reads before it drops the read ones. It
	// is also the most rows one call returns, so a caller that gates the
	// rows sees every unread row the scan found.
	MaxAddressedScan = 2000
)

// MentionForm says how a mention target names its principal. The form is
// decided once, when the message is posted, and stored, so a later change
// of configuration does not reinterpret an old message.
type MentionForm string

// Mention forms.
const (
	// FormUser is a person entity id, the principal key where the ACL maps
	// logins to person entities.
	FormUser MentionForm = "user"
	// FormJob is a job identity, such as integration:triage-bot.
	FormJob MentionForm = "job"
	// FormLogin is a raw login, where the ACL maps no person entity.
	FormLogin MentionForm = "login"
)

// Mention is one principal a message addresses.
type Mention struct {
	// Target is the principal key: the canonical person entity id, the job
	// identity or the login.
	Target string `json:"target"`
	// Form is how Target names the principal.
	Form MentionForm `json:"form"`
	// Span is the span text as written, when it differs from Target, so a
	// renderer can match the span the author typed.
	Span string `json:"span,omitempty"`
}

// AddressKind says why a row addresses its target.
type AddressKind string

// Address kinds.
const (
	// AddressMention is a row for a target the message mentions.
	AddressMention AddressKind = "mention"
	// AddressReply is a row for the author of the message a reply answers.
	AddressReply AddressKind = "reply"
	// AddressAny asks [Store.ListAddressed] for rows of every kind.
	AddressAny AddressKind = ""
)

// Addressed is one addressed row as [Store.ListAddressed] returns it. It
// carries no message content.
type Addressed struct {
	// Channel is the channel as stored, for the caller's gate.
	Channel Channel
	// Seq is the addressing message.
	Seq int64
	// At is when the row was written: the post, or the edit that added
	// the mention.
	At time.Time
	// Kind is why the row addresses its target.
	Kind AddressKind
	// Author is the addressing message's author.
	Author Author
	// Type is the addressing message's type.
	Type string
	// ReplyTo is the parent seq, for a reply.
	ReplyTo int64
}

// CheckMentions validates the targets of a message and returns them
// deduplicated by target and sorted. A target that is empty, over
// [MaxTextBytes], not UTF-8, holding NUL, starting or ending in space, or
// of an unknown form is [ErrInvalidMessage]; more than [MaxMentions]
// distinct targets is [ErrTooLarge].
//
// Nil: returned for no targets.
func CheckMentions(ms []Mention) ([]Mention, error) {
	if len(ms) == 0 {
		return nil, nil
	}
	seen := make(map[string]struct{}, len(ms))
	out := make([]Mention, 0, len(ms))
	for _, m := range ms {
		if err := checkMentionText(m.Target); err != nil {
			return nil, err
		}
		if m.Span != "" {
			if err := checkMentionText(m.Span); err != nil {
				return nil, err
			}
		}
		switch m.Form {
		case FormUser, FormJob, FormLogin:
		default:
			return nil, fmt.Errorf("%w: mention form %q", ErrInvalidMessage, m.Form)
		}
		if _, dup := seen[m.Target]; dup {
			continue
		}
		seen[m.Target] = struct{}{}
		if m.Span == m.Target {
			m.Span = ""
		}
		out = append(out, m)
	}
	if len(out) > MaxMentions {
		return nil, fmt.Errorf("%w: %d mention targets, at most %d", ErrTooLarge, len(out), MaxMentions)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Target < out[j].Target })
	return out, nil
}

func checkMentionText(s string) error {
	if s == "" || len(s) > MaxTextBytes || !utf8.ValidString(s) || strings.ContainsRune(s, 0) ||
		strings.TrimSpace(s) != s {

		return fmt.Errorf("%w: mention target is empty, over %d bytes, not UTF-8, holding NUL or "+
			"surrounded by space", ErrInvalidMessage, MaxTextBytes)
	}
	return nil
}

// MentionTargets returns the targets of ms.
//
// Nil: returned for no mentions.
func MentionTargets(ms []Mention) []string {
	if len(ms) == 0 {
		return nil
	}
	out := make([]string, len(ms))
	for i, m := range ms {
		out[i] = m.Target
	}
	return out
}

// ReplyTarget returns the user a reply by poster to parent addresses: the
// parent author's user. It reports false when the parent was posted by
// the trigger engine, a timer or a reminder (TriggeredBy set), when its
// author user is empty, [principal.Unknown] or one of rela's own
// identities, or when it is the poster's own user. The comparison is on
// the user only, because a row names a user only: a reply by the same
// user name from another organization addresses no one.
func ReplyTarget(parent Message, poster Author) (string, bool) {
	u := parent.Author.User
	if parent.TriggeredBy != "" || strings.TrimSpace(u) == "" || u == principal.Unknown ||
		principal.IsRelaIdentity(u) || u == poster.User {

		return "", false
	}
	return u, true
}

// CheckListAddressed validates the arguments of [Store.ListAddressed]: a
// valid reader, a kind that is [AddressMention], [AddressReply] or
// [AddressAny], and a limit from 1 to [MaxAddressedScan], each failure
// [ErrInvalidMessage].
func CheckListAddressed(r Reader, kind AddressKind, limit int) error {
	if err := r.Validate(); err != nil {
		return err
	}
	switch kind {
	case AddressAny, AddressMention, AddressReply:
	default:
		return fmt.Errorf("%w: unknown address kind %q", ErrInvalidMessage, kind)
	}
	if limit < 1 || limit > MaxAddressedScan {
		return fmt.Errorf("%w: addressed limit %d is not from 1 to %d", ErrInvalidMessage, limit,
			MaxAddressedScan)
	}
	return nil
}

// Admits reports whether a row of kind k is asked for by kind, which may
// be [AddressAny].
func (kind AddressKind) Admits(k AddressKind) bool { return kind == AddressAny || kind == k }

// SortAddressed sorts as newest first: time descending, then [KeyLess],
// then seq ascending, then mention before reply. A backend that sorts in Go
// uses it, so the backends agree.
func SortAddressed(as []Addressed) {
	sort.SliceStable(as, func(i, j int) bool {
		a, b := as[i], as[j]
		if !a.At.Equal(b.At) {
			return a.At.After(b.At)
		}
		if a.Channel.Key != b.Channel.Key {
			return KeyLess(a.Channel.Key, b.Channel.Key)
		}
		if a.Seq != b.Seq {
			return a.Seq < b.Seq
		}
		return a.Kind == AddressMention && b.Kind != AddressMention
	})
}

// AddressedRow is one stored addressed row as the cap step sees it.
type AddressedRow struct {
	Seq    int64
	Target string
	Kind   AddressKind
	// Author is the addressing message's author user.
	Author string
}

// Evict picks the row a full set frees for a new row whose message is by
// author. rows is sorted by seq, oldest first. markers returns the seqs of
// every read marker the target user holds on the channel, one per
// organization.
//
// The order: the oldest row that is read under every marker its target
// holds (at least one marker must exist); else the oldest row whose message
// has the same author user, which is unread for some marker; else none, and
// the new row is not written. One poster can therefore never erase another
// poster's unread row.
func Evict(rows []AddressedRow, markers func(target string) []int64, author string) (int, bool) {
	floor := make(map[string]int64)
	for i, r := range rows {
		low, ok := floor[r.Target]
		if !ok {
			ms := markers(r.Target)
			low = -1
			if len(ms) > 0 {
				low = slices.Min(ms)
			}
			floor[r.Target] = low
		}
		if r.Seq <= low {
			return i, true
		}
	}
	for i, r := range rows {
		if r.Author == author {
			return i, true
		}
	}
	return -1, false
}

// AppendRows returns the rows an append of m writes: one mention row per
// target of m.Mentions, then a reply row for replyTarget when it is set.
// A backend keeps or skips each through the cap step.
//
// Nil: returned when m addresses no one.
func AppendRows(m Message, replyTarget string) []AddressedRow {
	var out []AddressedRow
	for _, t := range m.Mentions {
		out = append(out, AddressedRow{Seq: m.Seq, Target: t.Target, Kind: AddressMention, Author: m.Author.User})
	}
	if replyTarget != "" {
		out = append(out, AddressedRow{Seq: m.Seq, Target: replyTarget, Kind: AddressReply, Author: m.Author.User})
	}
	return out
}

// AddedMentions returns the targets of next that old does not hold.
//
// Nil: returned when nothing was added.
func AddedMentions(old, next []Mention) []Mention {
	held := make(map[string]struct{}, len(old))
	for _, m := range old {
		held[m.Target] = struct{}{}
	}
	var out []Mention
	for _, m := range next {
		if _, ok := held[m.Target]; !ok {
			out = append(out, m)
		}
	}
	return out
}

// LogSkippedRow records a row the cap step did not write because the
// target's set is full of other posters' unread rows. It names the
// channel, the seq and the target, never content.
func LogSkippedRow(k Key, seq int64, target string) {
	slog.Warn("channels: a mention target's rows in this channel are full of unread rows by other posters; "+
		"the new row is not written", "channel", k.String(), "seq", seq, "target", target)
}
