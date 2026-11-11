package remotesync

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/canonical"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/state"
)

// The conflict list is one state.KV document per remote, beside the
// ledger. It holds what waits for a person: a field both sides changed since
// the last sync that a rule held, and a push the remote refused. Each such
// entry stays until the next sync finds the two sides agree again or someone
// resolves it with [Resolve]. It also keeps the value the latest rule
// overwrote, in a superseded entry that blocks nothing: one per field, a
// newer one replacing it, until someone dismisses it with [Dismiss] or the
// row goes. A status page reads the list with [LoadConflicts] without
// loading the ledger.

// conflictsVersion is the document format.
const conflictsVersion = 1

// The reasons an entry is on the list.
const (
	// ReasonBothChanged: both sides changed the field since the last sync.
	ReasonBothChanged = "both_changed"
	// ReasonRefused: the remote refused a push of the row (400, 403, 409,
	// 422).
	ReasonRefused = "refused"
	// ReasonUncertain: a create failed without saying whether the remote
	// made the row: the run was cancelled or timed out after the request
	// left, the connection broke, the remote answered 5xx, 429 or 503, or
	// redirected, or a 2xx answer named no new row. It is not sent again
	// until someone checks the remote and resolves it to the local side.
	ReasonUncertain = "uncertain"
	// ReasonSuperseded: the latest rule decided the field, and the entry
	// keeps the losing value. It blocks nothing.
	ReasonSuperseded = "superseded"
)

// KindRelation marks an entry about a relation: Local is then the local
// relation key and Type the local relation type.
const KindRelation = "relation"

// The sides a conflict is resolved to.
const (
	TakeLocal  = "local"
	TakeRemote = "remote"
)

// maxHeldValue caps the JSON size of a remote value an entry keeps. A larger
// value is left out and the entry says so.
const maxHeldValue = 16 << 10

// maxDetail caps the remote's explanation of a refusal.
const maxDetail = 512

// Conflict is one entry on a remote's conflict list.
type Conflict struct {
	// Kind is empty for an entity and [KindRelation] for a relation.
	Kind string `json:"kind,omitempty"`
	// Local is the local entity id, or the local relation key, and Type its
	// type.
	Local string `json:"local"`
	Type  string `json:"type"`
	// Address is the remote row, as the reconcile diff names it. It is
	// empty for a new row the remote refused to create.
	Address string `json:"address,omitempty"`
	// Field is the local field, and RemoteField the remote field it maps
	// to. Both are empty when the whole row is held.
	Field       string `json:"field,omitempty"`
	RemoteField string `json:"remote_field,omitempty"`
	// Reason is [ReasonBothChanged], [ReasonRefused], [ReasonUncertain] or
	// [ReasonSuperseded].
	Reason string `json:"reason"`
	// Detail explains a refusal, as the remote put it.
	Detail string `json:"detail,omitempty"`
	// Rule is the field's conflict rule.
	Rule string `json:"rule"`
	// Fallback says why a field under the latest rule was held instead of
	// decided: [FallbackTimeWithheld], [FallbackTimeUntrusted] or
	// [FallbackSameTime].
	Fallback string `json:"fallback,omitempty"`
	// Since is when the entry was first held, and Seen when a sync last
	// found it still in conflict.
	Since time.Time `json:"since"`
	Seen  time.Time `json:"seen"`
	// RemoteValue is the JSON encoding of the remote's value of Field when
	// it was last fetched, unless it was larger than the list keeps
	// (RemoteValueOmitted). The local value of a held field is the row's
	// own; the list does not copy it.
	RemoteValue        json.RawMessage `json:"remote_value,omitempty"`
	RemoteValueOmitted bool            `json:"remote_value_omitted,omitempty"`
	// LocalDigest and RemoteDigest are the digests of the two values when
	// the entry was last seen. Resolving sets the field's merge base to the
	// losing side's digest, so the next sync sends the winner. A refused
	// push keeps the digests of the local values it tried to send, keyed by
	// the ledger's base key.
	LocalDigest   string            `json:"local_digest,omitempty"`
	RemoteDigest  string            `json:"remote_digest,omitempty"`
	RefusedFields map[string]string `json:"refused_fields,omitempty"`
	// Superseded is set on a [ReasonSuperseded] entry.
	Superseded *Superseded `json:"superseded,omitempty"`
}

// Superseded is what the latest rule decided for a field: the side that
// won, both sides' last-updated times, and the losing value. A losing local
// value is kept here; a losing remote value is the entry's RemoteValue.
// Either is left out when larger than the list keeps.
type Superseded struct {
	Winner            string          `json:"winner"`
	LocalValue        json.RawMessage `json:"local_value,omitempty"`
	LocalValueOmitted bool            `json:"local_value_omitted,omitempty"`
	LocalUpdated      time.Time       `json:"local_updated"`
	RemoteUpdated     time.Time       `json:"remote_updated"`
}

// key identifies an entry: the local row and the field, and for a
// superseded entry a suffix, so it sits beside a held entry of the field.
func (c *Conflict) key() string {
	if c.Reason == ReasonSuperseded {
		return supersededKey(c.Local, c.Field)
	}
	return conflictKey(c.Local, c.Field)
}

func conflictKey(local, field string) string { return local + "\x00" + field }

func supersededKey(local, field string) string { return conflictKey(local, field) + "\x00s" }

type conflictsDoc struct {
	Version   int         `json:"version"`
	Conflicts []*Conflict `json:"conflicts"`
}

// conflicts is the loaded conflict list of one remote.
type conflicts struct {
	key     string
	entries map[string]*Conflict
}

func conflictsKey(remote string) string { return "remotesync/" + remote + "/conflicts.json" }

// loadConflicts reads the conflict list of remote name. A missing list is
// empty.
func loadConflicts(ctx context.Context, kv state.KV, name string) (*conflicts, error) {
	c := &conflicts{key: conflictsKey(name), entries: map[string]*Conflict{}}
	data, err := kv.Get(ctx, c.key)
	switch {
	case errors.Is(err, fs.ErrNotExist) || os.IsNotExist(err):
		return c, nil
	case err != nil:
		return nil, fmt.Errorf("remotesync: read conflict list: %w", err)
	}
	var doc conflictsDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("remotesync: the conflict list %s is corrupt: %w", c.key, err)
	}
	if doc.Version != conflictsVersion {
		return nil, fmt.Errorf("remotesync: the conflict list %s has version %d, want %d",
			c.key, doc.Version, conflictsVersion)
	}
	for _, e := range doc.Conflicts {
		if e == nil || e.Local == "" || !validReason(e.Reason) || (e.Reason == ReasonSuperseded) != (e.Superseded != nil) {
			return nil, fmt.Errorf("remotesync: the conflict list %s holds an invalid entry", c.key)
		}
		c.entries[e.key()] = e
	}
	return c, nil
}

func validReason(r string) bool {
	return r == ReasonBothChanged || r == ReasonRefused || r == ReasonUncertain || r == ReasonSuperseded
}

// list returns the entries sorted by local id, then field, then reason.
func (c *conflicts) list() []Conflict {
	out := make([]Conflict, 0, len(c.entries))
	for _, e := range c.entries {
		out = append(out, *e)
	}
	slices.SortFunc(out, func(a, b Conflict) int {
		return cmp.Or(cmp.Compare(a.Local, b.Local), cmp.Compare(a.Field, b.Field), cmp.Compare(a.Reason, b.Reason))
	})
	return out
}

func (c *conflicts) save(ctx context.Context, kv state.KV) error {
	doc := conflictsDoc{Version: conflictsVersion, Conflicts: make([]*Conflict, 0, len(c.entries))}
	for _, e := range c.list() {
		doc.Conflicts = append(doc.Conflicts, &e)
	}
	data, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("remotesync: encode conflict list: %w", err)
	}
	if err := kv.Put(ctx, c.key, data); err != nil {
		return fmt.Errorf("remotesync: write conflict list: %w", err)
	}
	return nil
}

// hold puts e on the list, or refreshes the entry already there: an entry
// keeps the time it was first held. An entry without a rule gets hold.
func (c *conflicts) hold(e *Conflict, now time.Time) {
	e.Rule, e.Seen = cmp.Or(e.Rule, ConflictHold), now
	e.Since = now
	if old, ok := c.entries[e.key()]; ok {
		e.Since = old.Since
	}
	c.entries[e.key()] = e
}

// supersede keeps the value the latest rule overwrote, replacing an older
// superseded entry of the same field.
func (c *conflicts) supersede(e *Conflict, now time.Time) {
	e.Reason, e.Rule, e.Since, e.Seen = ReasonSuperseded, ConflictLatest, now, now
	c.entries[e.key()] = e
}

// expire drops the superseded entries first kept more than retention ago.
// A retention of 0 keeps no local value, so its entries hold only digests
// and are not dropped by age.
func (c *conflicts) expire(now time.Time, retention time.Duration) {
	if retention <= 0 {
		return
	}
	for k, e := range c.entries {
		if e.Reason == ReasonSuperseded && now.Sub(e.Since) > retention {
			delete(c.entries, k)
		}
	}
}

// purge drops every superseded entry of the local entity id, including the
// entries of relations it is an endpoint of, and reports how many.
func (c *conflicts) purge(id string) int {
	n := 0
	for k, e := range c.entries {
		if e.Reason == ReasonSuperseded && entryNames(e, id) {
			delete(c.entries, k)
			n++
		}
	}
	return n
}

// entryNames reports whether entry e is about the local entity id: its row,
// or a relation with id at either end.
func entryNames(e *Conflict, id string) bool {
	if e.Kind != KindRelation {
		return e.Local == id
	}
	// A relation entry's Local is [entity.RelationKey.String]: FROM--type--TO.
	parts := strings.SplitN(e.Local, "--", 3)
	if len(parts) != 3 {
		return false
	}
	from, _, err := entity.ParseStateRef(parts[0])
	return err == nil && (from == id || parts[2] == id)
}

// PurgeSuperseded drops the superseded entries of the local entity id from
// the conflict list of remote name, with the losing values they keep. A
// history purge calls it for every remote, so a purged value leaves no copy
// here. It returns how many entries it dropped. The caller holds the
// remote's run lock.
func PurgeSuperseded(ctx context.Context, kv state.KV, name, id string) (int, error) {
	c, err := loadConflicts(ctx, kv, name)
	if err != nil {
		return 0, err
	}
	n := c.purge(id)
	if n == 0 {
		return 0, nil
	}
	return n, c.save(ctx, kv)
}

// drop removes the held entry of a local row's field. A superseded entry
// stays.
func (c *conflicts) drop(local, field string) { delete(c.entries, conflictKey(local, field)) }

// holdsField reports whether a field of the local row local, or the whole
// row, is held.
func (c *conflicts) holdsField(local string) bool {
	for _, e := range c.entries {
		if e.Local == local && e.Reason != ReasonSuperseded {
			return true
		}
	}
	return false
}

// blocked reports whether a push of the local row's field waits on the
// list: the field or the whole row is held.
func (c *conflicts) blocked(local, field string) bool {
	_, row := c.entries[conflictKey(local, "")]
	_, f := c.entries[conflictKey(local, field)]
	return row || f
}

// heldValue returns the JSON encoding of v as an entry keeps it, and whether
// it was left out for its size ([maxHeldValue]).
func heldValue(v any) (json.RawMessage, bool) {
	b, err := json.Marshal(v)
	if err != nil || len(b) > maxHeldValue {
		return nil, true
	}
	return b, false
}

// LoadConflicts returns the conflict list of remote name, sorted by local id
// and field. A remote that never held a conflict has an empty list.
//
// The entries hold remote values and local ids outside the graph, so they
// are not filtered by this project's acl.yaml. A caller that shows them to
// anyone but an operator must gate each entry by that reader's access to
// the local row and field.
func LoadConflicts(ctx context.Context, kv state.KV, name string) ([]Conflict, error) {
	c, err := loadConflicts(ctx, kv, name)
	if err != nil {
		return nil, err
	}
	return c.list(), nil
}

// ErrNoConflict reports a resolve that named no entry on the list.
var ErrNoConflict = errors.New("remotesync: no such conflict")

// CheckResolve returns the entries of list that a resolve of the local row
// local, all of its entries or the one of field, to side take would
// resolve. It fails with [ErrNoConflict] when there is none, and for a
// side an entry cannot take. A caller checks before it takes the run lock,
// so a mistyped resolve does not count as a run.
func CheckResolve(list []Conflict, local, field, take string) ([]Conflict, error) {
	if take != TakeLocal && take != TakeRemote {
		return nil, fmt.Errorf("remotesync: resolve to %q: use %s or %s", take, TakeLocal, TakeRemote)
	}
	var out []Conflict
	for _, e := range list {
		if e.Local != local || (field != "" && e.Field != field) {
			continue
		}
		if e.Reason == ReasonSuperseded {
			if field != "" {
				return nil, fmt.Errorf("remotesync: %s %s: the latest rule already decided this field; "+
					"dismiss the entry instead", e.Local, e.Field)
			}
			continue
		}
		if e.Reason != ReasonBothChanged && e.Address == "" && take == TakeRemote {
			return nil, fmt.Errorf("remotesync: %s: the remote refused to create the row, so there is no remote "+
				"value to take; resolve to %s to try again, or narrow share to stop pushing it", e.Local, TakeLocal)
		}
		out = append(out, e)
	}
	if len(out) == 0 {
		return nil, ErrNoConflict
	}
	return out, nil
}

// Resolve resolves the entries of the local row local on m's conflict
// list, all of them or the one of field, to side take ([TakeLocal] or
// [TakeRemote]). It writes neither side. It sets each field's merge base to
// the losing side's value and marks the row to be compared again, so the
// next sync sends the winning value, by the normal write path on each side.
// A refused push resolved to the local side is tried again; resolved to
// the remote side, the local values it tried to send stop counting as
// changes and the next sync brings the remote's values.
//
// The caller holds the remote's run lock, so no sync rewrites the ledger
// meanwhile. It returns how many entries it resolved.
func Resolve(ctx context.Context, kv state.KV, m *Mapping, local, field, take string) (int, error) {
	if m == nil {
		return 0, errors.New("remotesync: a mapping is required")
	}
	l, err := loadLedger(ctx, kv, m)
	if err != nil {
		return 0, err
	}
	c, err := loadConflicts(ctx, kv, m.Name)
	if err != nil {
		return 0, err
	}
	entries, err := CheckResolve(c.list(), local, field, take)
	if err != nil {
		return 0, err
	}
	kind := "E"
	if entries[0].Kind == KindRelation {
		kind = "R"
	}
	var row *ledgerRow
	for _, r := range l.rows {
		if r.Kind == kind && r.Local == local && r.State == stateLinked {
			row = r
		}
	}
	for _, e := range entries {
		resolveOne(row, &e, take)
		c.drop(e.Local, e.Field)
	}
	if err := l.save(ctx, kv, m); err != nil {
		return 0, err
	}
	return len(entries), c.save(ctx, kv)
}

// resolveOne moves row's merge base for entry e to the losing side.
// [CheckResolve] has checked that e can take that side.
func resolveOne(row *ledgerRow, e *Conflict, take string) {
	switch {
	case e.Reason != ReasonBothChanged && take == TakeLocal:
		// Tried again on the next sync.
		return
	case row == nil:
		// The row is no longer linked; the entry has nothing to apply to.
		return
	}
	if row.Base == nil {
		row.Base = map[string]string{}
	}
	switch {
	case e.Reason != ReasonBothChanged:
		maps.Copy(row.Base, e.RefusedFields)
	case take == TakeLocal:
		row.Base[baseKeyOf(e.Field, e.RemoteField)] = e.RemoteDigest
	default:
		row.Base[baseKeyOf(e.Field, e.RemoteField)] = e.LocalDigest
	}
	row.Hash = canonical.Digest{}
}

// CheckDismiss returns the superseded entries of list that a dismiss of the
// local row local, all of them or the one of field, would remove. It fails
// with [ErrNoConflict] when there is none, and when field names a held entry,
// which needs a resolve.
func CheckDismiss(list []Conflict, local, field string) ([]Conflict, error) {
	var out []Conflict
	held := false
	for _, e := range list {
		if e.Local != local || (field != "" && e.Field != field) {
			continue
		}
		if e.Reason != ReasonSuperseded {
			held = true
			continue
		}
		out = append(out, e)
	}
	switch {
	case len(out) > 0:
		return out, nil
	case held:
		return nil, fmt.Errorf("remotesync: %s: the entry is held, not superseded; resolve it with --take", local)
	}
	return nil, ErrNoConflict
}

// Dismiss removes the superseded entries of the local row local on m's
// conflict list, all of them or the one of field. It changes neither side
// and no merge base. The caller holds the remote's run lock. It returns how
// many entries it removed.
func Dismiss(ctx context.Context, kv state.KV, m *Mapping, local, field string) (int, error) {
	if m == nil {
		return 0, errors.New("remotesync: a mapping is required")
	}
	c, err := loadConflicts(ctx, kv, m.Name)
	if err != nil {
		return 0, err
	}
	entries, err := CheckDismiss(c.list(), local, field)
	if err != nil {
		return 0, err
	}
	for _, e := range entries {
		delete(c.entries, e.key())
	}
	return len(entries), c.save(ctx, kv)
}
