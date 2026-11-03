package realtime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/channels"
)

// A hint tells the other processes of a project that channels changed,
// and of what kind. It carries channel keys and principal keys, never
// channel content: the receiver replays its subscriptions as each
// subscriber, as it does for a local write, so a hint cannot show anything
// the subscriber could not already read.
//
// A payload is one JSON object: the sending [Local]'s origin and a list of
// hints. Each hint has one of five kinds:
//
//   - hintMessage, hintMoved: channel keys, marked with that kind.
//   - hintAddressed: one channel key, its kind, and the principal keys the
//     post or edit addressed (one markAddressed call).
//   - hintRead: one reader and the keys whose read marker moved (one
//     markFor call).
//   - hintResync: every subscriber rechecks every channel.

// HintSender sends one hint payload to the other processes of the project.
// It is best effort: an error is logged by the caller and never fails a
// write.
type HintSender interface {
	SendHint(ctx context.Context, payload []byte) error
}

// HintSource follows the hint payloads other processes send. FollowHints
// calls gap once it is positioned, so that every payload sent from then on
// reaches deliver: what was sent before is the caller's to recheck. After
// that it calls deliver for each payload, and gap again when payloads may
// have been lost (a reconnect, a rotated or truncated journal, an unusable
// line). Neither may block for long: they run on the source's own
// goroutine. It returns when ctx ends, or with an error when it cannot
// follow at all.
type HintSource interface {
	FollowHints(ctx context.Context, deliver func(payload []byte), gap func()) error
}

// Hint kinds on the wire.
const (
	hintMessage   = "m"
	hintMoved     = "v"
	hintAddressed = "a"
	hintRead      = "r"
	hintResync    = "s"
)

// Hint caps. A mark past a cap becomes a resync hint. maxHintBytes stays
// under PostgreSQL's 8000-byte NOTIFY payload with room for the schema
// prefix.
const (
	maxHintKeys    = 64
	maxHintTargets = 64
	maxHintBytes   = 7000
)

// hintPayload is the wire form of one send.
type hintPayload struct {
	Origin string     `json:"o"`
	Hints  []wireHint `json:"h"`
}

// wireHint is one hint. Which fields are set depends on Kind; see the
// package comment above.
type wireHint struct {
	Kind string         `json:"k"`
	Keys []channels.Key `json:"c,omitempty"`
	// Of is the mark kind of an addressed hint: hintMessage or hintMoved.
	Of      string   `json:"of,omitempty"`
	Targets []string `json:"t,omitempty"`
	User    string   `json:"u,omitempty"`
	Org     string   `json:"g,omitempty"`
}

// errBadHint is the cause of a payload that does not decode. The receiver
// treats it as a gap, never as partial data.
var errBadHint = errors.New("realtime: unusable channel hint")

// newOrigin returns a random id for one [Local], so it can drop its own
// hints.
func newOrigin() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil { // coverage-ignore: crypto/rand does not fail on supported platforms
		panic(fmt.Sprintf("realtime: origin id: %v", err))
	}
	return hex.EncodeToString(b[:])
}

// decodeHints parses and validates a payload. Every key, reader and
// target is checked with the channels validators; any failure rejects the
// whole payload.
func decodeHints(payload []byte) (hintPayload, error) {
	if len(payload) > maxHintBytes {
		return hintPayload{}, fmt.Errorf("%w: %d bytes", errBadHint, len(payload))
	}
	var p hintPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return hintPayload{}, fmt.Errorf("%w: %w", errBadHint, err)
	}
	if p.Origin == "" {
		return hintPayload{}, fmt.Errorf("%w: no origin", errBadHint)
	}
	for _, h := range p.Hints {
		if err := h.validate(); err != nil {
			return hintPayload{}, err
		}
	}
	return p, nil
}

// validate checks one decoded hint.
func (h wireHint) validate() error {
	switch h.Kind {
	case hintResync:
		return nil
	case hintMessage, hintMoved:
		return validKeys(h.Keys, 1, maxHintKeys)
	case hintAddressed:
		if h.Of != hintMessage && h.Of != hintMoved {
			return fmt.Errorf("%w: addressed kind %q", errBadHint, h.Of)
		}
		if len(h.Targets) == 0 || len(h.Targets) > maxHintTargets {
			return fmt.Errorf("%w: %d targets", errBadHint, len(h.Targets))
		}
		for _, t := range h.Targets {
			// A target is a principal key: the reader rules hold for it.
			if err := (channels.Reader{User: t}).Validate(); err != nil {
				return fmt.Errorf("%w: %w", errBadHint, err)
			}
		}
		return validKeys(h.Keys, 1, 1)
	case hintRead:
		if err := (channels.Reader{User: h.User, OrgID: h.Org}).Validate(); err != nil {
			return fmt.Errorf("%w: %w", errBadHint, err)
		}
		return validKeys(h.Keys, 1, maxHintKeys)
	default:
		return fmt.Errorf("%w: kind %q", errBadHint, h.Kind)
	}
}

// validKeys checks that keys holds from lo to hi valid channel keys.
func validKeys(keys []channels.Key, lo, hi int) error {
	if len(keys) < lo || len(keys) > hi {
		return fmt.Errorf("%w: %d keys", errBadHint, len(keys))
	}
	for _, k := range keys {
		if err := k.Validate(); err != nil {
			return fmt.Errorf("%w: %w", errBadHint, err)
		}
	}
	return nil
}

// hintBatch is the marks queued in one merge window, merged: one set of
// keys per kind, one entry per addressed key and per reader.
type hintBatch struct {
	resync    bool
	message   map[channels.Key]struct{}
	moved     map[channels.Key]struct{}
	addressed map[channels.Key]*addressedMarks
	reads     map[channels.Reader]map[channels.Key]struct{}
	// order keeps addressed keys and readers in arrival order, so the
	// payloads of one batch are stable.
	addrOrder []channels.Key
	readOrder []channels.Reader
}

// addressedMarks is the merged addressed marks of one channel key.
type addressedMarks struct {
	moved   bool
	targets map[string]struct{}
}

func newHintBatch() *hintBatch {
	return &hintBatch{
		message:   map[channels.Key]struct{}{},
		moved:     map[channels.Key]struct{}{},
		addressed: map[channels.Key]*addressedMarks{},
		reads:     map[channels.Reader]map[channels.Key]struct{}{},
	}
}

// add merges one queued entry into b.
func (b *hintBatch) add(e hintEntry) {
	switch {
	case e.resync:
		b.resync = true
	case e.read:
		set := b.reads[e.reader]
		if set == nil {
			set = map[channels.Key]struct{}{}
			b.reads[e.reader] = set
			b.readOrder = append(b.readOrder, e.reader)
		}
		for _, k := range e.keys {
			set[k] = struct{}{}
		}
	case len(e.targets) > 0:
		k := e.keys[0]
		a := b.addressed[k]
		if a == nil {
			a = &addressedMarks{targets: map[string]struct{}{}}
			b.addressed[k] = a
			b.addrOrder = append(b.addrOrder, k)
		}
		a.moved = a.moved || e.kind == markKindMoved
		for _, t := range e.targets {
			a.targets[t] = struct{}{}
		}
	case e.kind == markKindMoved:
		for _, k := range e.keys {
			b.moved[k] = struct{}{}
		}
	default:
		for _, k := range e.keys {
			b.message[k] = struct{}{}
		}
	}
}

// hints returns the merged hints of b. A hint past a cap becomes a resync,
// and a resync replaces every other hint: the receiver rechecks every
// channel anyway.
func (b *hintBatch) hints() []wireHint {
	if b.resync {
		return []wireHint{{Kind: hintResync}}
	}
	var out []wireHint
	// A key marked moved is in the durable set too, so it need not be sent
	// as a message as well.
	for k := range b.moved {
		delete(b.message, k)
	}
	for _, kind := range []string{hintMoved, hintMessage} {
		set := b.moved
		if kind == hintMessage {
			set = b.message
		}
		if len(set) == 0 {
			continue
		}
		if len(set) > maxHintKeys {
			return []wireHint{{Kind: hintResync}}
		}
		out = append(out, wireHint{Kind: kind, Keys: sortedChannelKeys(set)})
	}
	for _, k := range b.addrOrder {
		a := b.addressed[k]
		if len(a.targets) > maxHintTargets {
			return []wireHint{{Kind: hintResync}}
		}
		of := hintMessage
		if a.moved {
			of = hintMoved
		}
		targets := make([]string, 0, len(a.targets))
		for t := range a.targets {
			targets = append(targets, t)
		}
		slices.Sort(targets)
		out = append(out, wireHint{Kind: hintAddressed, Keys: []channels.Key{k}, Of: of, Targets: targets})
	}
	for _, r := range b.readOrder {
		set := b.reads[r]
		if len(set) > maxHintKeys {
			return []wireHint{{Kind: hintResync}}
		}
		out = append(out, wireHint{Kind: hintRead, Keys: sortedChannelKeys(set), User: r.User, Org: r.OrgID})
	}
	return out
}

func sortedChannelKeys(set map[channels.Key]struct{}) []channels.Key {
	out := make([]channels.Key, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	channels.SortKeys(out)
	return out
}

// encodeHints packs hs into payloads of at most maxHintBytes each. A single
// hint that does not fit alone turns the whole send into one resync.
func encodeHints(origin string, hs []wireHint) [][]byte {
	resync := func() [][]byte {
		b, err := json.Marshal(hintPayload{Origin: origin, Hints: []wireHint{{Kind: hintResync}}})
		if err != nil { // coverage-ignore: a fixed struct of strings always marshals
			panic(err)
		}
		return [][]byte{b}
	}
	var out [][]byte
	var cur []wireHint
	var last []byte
	for _, h := range hs {
		next := append(slices.Clip(cur), h)
		b, err := json.Marshal(hintPayload{Origin: origin, Hints: next})
		if err != nil { // coverage-ignore: keys and strings always marshal
			return resync()
		}
		if len(b) <= maxHintBytes {
			cur, last = next, b
			continue
		}
		if len(cur) == 0 {
			return resync()
		}
		out = append(out, last)
		cur = []wireHint{h}
		if last, err = json.Marshal(hintPayload{Origin: origin, Hints: cur}); err != nil || len(last) > maxHintBytes {
			return resync()
		}
	}
	if len(cur) > 0 {
		out = append(out, last)
	}
	return out
}
