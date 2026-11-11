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
	"time"

	"github.com/Sourcehaven-BV/rela/internal/canonical"
	"github.com/Sourcehaven-BV/rela/internal/state"
)

// The conflict list is one state.KV document per remote, beside the
// ledger. It holds what the hold rule keeps waiting for a person: a field
// both sides changed since the last sync, and a push the remote refused.
// Each entry stays until the next sync finds the two sides agree again or
// someone resolves it with [Resolve]. A status page reads it with
// [LoadConflicts] without loading the ledger.

// conflictsVersion is the document format.
const conflictsVersion = 1

// The reasons an entry is on the list.
const (
	// ReasonBothChanged: both sides changed the field since the last sync.
	ReasonBothChanged = "both_changed"
	// ReasonRefused: the remote refused a push of the row (403, 409, 422).
	ReasonRefused = "refused"
	// ReasonUncertain: a create failed without saying whether the remote
	// made the row (a broken connection or a 5xx). It is not sent again
	// until someone checks the remote and resolves it to the local side.
	ReasonUncertain = "uncertain"
)

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
	// Local is the local entity id and Type its type.
	Local string `json:"local"`
	Type  string `json:"type"`
	// Address is the remote row, as the reconcile diff names it. It is
	// empty for a new row the remote refused to create.
	Address string `json:"address,omitempty"`
	// Field is the local field, and RemoteField the remote field it maps
	// to. Both are empty when the whole row is held.
	Field       string `json:"field,omitempty"`
	RemoteField string `json:"remote_field,omitempty"`
	// Reason is [ReasonBothChanged], [ReasonRefused] or [ReasonUncertain].
	Reason string `json:"reason"`
	// Detail explains a refusal, as the remote put it.
	Detail string `json:"detail,omitempty"`
	// Rule is the conflict rule that held the entry.
	Rule string `json:"rule"`
	// Since is when the entry was first held, and Seen when a sync last
	// found it still in conflict.
	Since time.Time `json:"since"`
	Seen  time.Time `json:"seen"`
	// RemoteValue is the JSON encoding of the remote's value of Field when
	// it was last fetched, unless it was larger than the list keeps
	// (RemoteValueOmitted). The local value is the row's own; the list does
	// not copy it.
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
}

// key identifies an entry: the local row and the field.
func (c *Conflict) key() string { return conflictKey(c.Local, c.Field) }

func conflictKey(local, field string) string { return local + "\x00" + field }

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
		if e == nil || e.Local == "" || !validReason(e.Reason) {
			return nil, fmt.Errorf("remotesync: the conflict list %s holds an invalid entry", c.key)
		}
		c.entries[e.key()] = e
	}
	return c, nil
}

func validReason(r string) bool {
	return r == ReasonBothChanged || r == ReasonRefused || r == ReasonUncertain
}

// list returns the entries sorted by local id, then field.
func (c *conflicts) list() []Conflict {
	out := make([]Conflict, 0, len(c.entries))
	for _, e := range c.entries {
		out = append(out, *e)
	}
	slices.SortFunc(out, func(a, b Conflict) int {
		return cmp.Or(cmp.Compare(a.Local, b.Local), cmp.Compare(a.Field, b.Field))
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
// keeps the time it was first held.
func (c *conflicts) hold(e *Conflict, now time.Time) {
	e.Rule, e.Seen = ConflictHold, now
	e.Since = now
	if old, ok := c.entries[e.key()]; ok {
		e.Since = old.Since
	}
	c.entries[e.key()] = e
}

// drop removes the entry of a local row's field.
func (c *conflicts) drop(local, field string) { delete(c.entries, conflictKey(local, field)) }

// blocked reports whether a push of the local row's field waits on the
// list: the field or the whole row is held.
func (c *conflicts) blocked(local, field string) bool {
	_, row := c.entries[conflictKey(local, "")]
	_, f := c.entries[conflictKey(local, field)]
	return row || f
}

// heldValue returns the JSON encoding of v as an entry keeps it, and whether
// it was left out for its size.
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
	var row *ledgerRow
	for _, r := range l.rows {
		if r.Kind == "E" && r.Local == local && r.State == stateLinked {
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
