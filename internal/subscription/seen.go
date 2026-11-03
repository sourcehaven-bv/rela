package subscription

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/feed"
	"github.com/Sourcehaven-BV/rela/internal/state"
)

// seenFactor sizes the seen-set relative to max_items. A feed rotates items
// out of its window; the set has to remember them for longer than the
// window, or an item that drops out and back in would be processed again.
const seenFactor = 4

// record is the persisted state of one subscription.
type record struct {
	// Config is a hash of the URL and mapping. A change resets the record:
	// validators of another URL mean nothing, and a fixed mapping should
	// see every item again. It also keeps the URL, which may carry a token
	// in its query, out of state.
	Config string `json:"config"`
	// Rebaseline is set when a record is reset, and cleared by the first
	// poll that completes. Until then an item whose entity already exists
	// is only marked seen: its then: steps ran under the old record, and
	// running them again would repeat every append and overwrite every set.
	Rebaseline   bool        `json:"rebaseline,omitempty"`
	ETag         string      `json:"etag,omitempty"`
	LastModified string      `json:"last_modified,omitempty"`
	Seen         []seenEntry `json:"seen,omitempty"` // least recently seen first
}

// seenEntry is one item that was imported or permanently rejected.
type seenEntry struct {
	ID      string `json:"id"`      // hash of the item's identity
	Content string `json:"content"` // hash of the item's fields
}

// stateKey is the state.KV key of a subscription. The name is hashed so any
// valid name is a valid key on every backend.
func stateKey(name string) string {
	sum := sha256.Sum256([]byte(name))
	return "subscriptions/" + hex.EncodeToString(sum[:16])
}

// configHash identifies the parts of a subscription the record depends on.
func configHash(sub Subscription) (string, error) {
	data, err := json.Marshal(struct {
		URL     string
		Mapping any
	}{sub.URL, sub.Mapping})
	if err != nil {
		return "", err
	}
	return hashOf(string(data)), nil
}

func loadRecord(ctx context.Context, kv state.KV, name, config string) (record, error) {
	data, err := kv.Get(ctx, stateKey(name))
	switch {
	case errors.Is(err, fs.ErrNotExist) || os.IsNotExist(err):
		return record{Config: config}, nil
	case err != nil:
		return record{}, fmt.Errorf("load state: %w", err)
	}
	var rec record
	if err := json.Unmarshal(data, &rec); err != nil || rec.Config != config {
		// A corrupt record costs one full poll; refusing to run would
		// stop the subscription until someone deletes state by hand.
		return record{Config: config, Rebaseline: true}, nil //nolint:nilerr // see above
	}
	return rec, nil
}

func saveRecord(ctx context.Context, kv state.KV, name string, rec record) error {
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	if err := kv.Put(ctx, stateKey(name), data); err != nil {
		return fmt.Errorf("save state: %w", err)
	}
	return nil
}

// seenSet is the in-memory form of record.Seen.
type seenSet struct {
	order []string          // identity hashes, least recently seen first
	byID  map[string]string // identity hash -> content hash
}

func newSeenSet(entries []seenEntry) *seenSet {
	s := &seenSet{byID: make(map[string]string, len(entries))}
	for _, e := range entries {
		if _, dup := s.byID[e.ID]; !dup {
			s.order = append(s.order, e.ID)
		}
		s.byID[e.ID] = e.Content
	}
	return s
}

// unchanged reports whether id was seen with this content.
func (s *seenSet) unchanged(id, content string) bool {
	c, ok := s.byID[id]
	return ok && c == content
}

// mark records id as seen now with content.
func (s *seenSet) mark(id, content string) {
	if _, ok := s.byID[id]; ok {
		s.order = removeString(s.order, id)
	}
	s.order = append(s.order, id)
	s.byID[id] = content
}

// touch moves an unchanged id to the most recently seen end, so an item
// that stays in the feed is never evicted.
func (s *seenSet) touch(id string) {
	s.mark(id, s.byID[id])
}

// entries returns the set, keeping the limit most recently seen.
func (s *seenSet) entries(limit int) []seenEntry {
	order := s.order
	if len(order) > limit {
		order = order[len(order)-limit:]
	}
	out := make([]seenEntry, len(order))
	for i, id := range order {
		out[i] = seenEntry{ID: id, Content: s.byID[id]}
	}
	return out
}

func removeString(list []string, v string) []string {
	for i, x := range list {
		if x == v {
			return append(list[:i], list[i+1:]...)
		}
	}
	return list
}

// identityHash hashes an item's match values in match order.
func identityHash(match []string, values map[string]string) string {
	var b strings.Builder
	for _, prop := range match {
		b.WriteString(prop)
		b.WriteByte(0)
		b.WriteString(values[prop])
		b.WriteByte(0)
	}
	return hashOf(b.String())
}

// contentHash hashes every field of an item a mapping can read.
func contentHash(f *feed.Feed, it feed.Item) string {
	parts := []string{
		f.Title, f.Link, it.ID, it.Title, it.Link, it.Summary, it.Content, it.PublishedRaw, it.UpdatedRaw,
		strings.Join(it.Authors, "\x00"), strings.Join(it.Categories, "\x00"),
	}
	return hashOf(strings.Join(parts, "\x01"))
}

func hashOf(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:16])
}
