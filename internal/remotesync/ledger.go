package remotesync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/canonical"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/reconcile"
	"github.com/Sourcehaven-BV/rela/internal/reconclient"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// The ledger is one state.KV document per remote. It records, per source row
// the pull has seen, the source view hash it last applied, the local row it
// maps to, and per owned field a digest of the value both sides last agreed
// on: the merge base. It lives outside the graph so every backend can pull,
// including fs projects without version history.

// ledgerVersion is the document format.
const ledgerVersion = 1

// The states of a ledger row.
const (
	// stateLinked: the local row mirrors the source row.
	stateLinked = "linked"
	// stateDetached: the source stopped listing the row and the local row
	// was kept. A detached row is left out of the diff, so the source
	// listing it again links it again.
	stateDetached = "detached"
	// stateRejected: the local schema refused the source row. It is tried
	// again when the source row changes.
	stateRejected = "rejected"
	// stateDropped: someone deleted the local row. The pull does not
	// recreate it.
	stateDropped = "dropped"
)

// ledgerRow is one source row.
type ledgerRow struct {
	// Kind is "E" or "R".
	Kind string `json:"k"`
	// Type is the source type.
	Type string `json:"t"`
	// Address is the source address, as the diff names it.
	Address string `json:"a"`
	// Hash is the source view hash last applied. The zero digest means the
	// row is compared again.
	Hash canonical.Digest `json:"h"`
	// Local is the local entity id, or the local relation key as
	// [entity.RelationKey.String] writes it.
	Local string `json:"l,omitempty"`
	// Face is the local entity's face. Mapped types are not faced, so it
	// is the implicit face; it is recorded so the ledger names the row it
	// read rather than assuming one.
	Face  entity.Face `json:"f,omitempty"`
	State string      `json:"s"`
	// Base holds, per owned field, the digest of the value both sides last
	// agreed on, keyed by [FieldMapping.baseKey].
	Base map[string]string `json:"b,omitempty"`
	// Adopted marks a local row that existed before the pull linked it. A
	// pull never deletes it; when the source stops listing it, it is
	// detached.
	Adopted bool `json:"ad,omitempty"`
}

// localRef is the local entity row an entity row maps to.
func (r *ledgerRow) localRef() entity.Ref { return entity.Ref{ID: r.Local, Face: r.Face} }

// adopt records the local entity's face, and whether the local row was
// there before the pull linked it. A row stays adopted once it is. A nil row
// (a dry run) is left alone.
func (r *ledgerRow) adopt(face entity.Face, adopted bool) {
	if r != nil {
		r.Face = face
		r.Adopted = r.Adopted || adopted
	}
}

func (r *ledgerRow) kind() reconcile.Kind {
	if r.Kind == "R" {
		return reconcile.KindRelation
	}
	return reconcile.KindEntity
}

// ledgerDoc is the stored form.
type ledgerDoc struct {
	Version int          `json:"version"`
	Mapping string       `json:"mapping"`
	Rows    []*ledgerRow `json:"rows"`
	// Held maps the rowKey of a source row that stopped without being
	// applied (a conflict, a failure, a deferred create) to the source hash
	// it stopped at. A run takes those rows last while their hash is the
	// same, so they cannot use up max_fetch ahead of other rows.
	Held map[string]canonical.Digest `json:"held,omitempty"`
	// Deletes holds the Unix times of the deletes in the last
	// [deleteWindow], for the gone guard.
	Deletes []int64 `json:"deletes,omitempty"`
}

// deleteWindow is how far back the gone guard counts a remote's deletes. A
// source that stops listing a few rows per run would otherwise delete in
// bulk over a day without ever tripping a per-run limit.
const deleteWindow = 24 * time.Hour

// ledger is the loaded ledger of one remote.
type ledger struct {
	key  string
	rows map[string]*ledgerRow // by rowKey
	// index holds the rows the diff sees, per kind, sorted by address.
	index   map[reconcile.Kind][]*ledgerRow
	held    map[string]canonical.Digest
	deletes []int64
}

func rowKey(kind reconcile.Kind, address string) string { return string(kind) + address }

func ledgerKey(remote string) string { return "remotesync/" + remote + "/ledger.json" }

// loadLedger reads the ledger of m's remote. A missing ledger is empty. A
// ledger written under another mapping keeps its rows and bases, but every
// hash is cleared, so every row is compared again.
func loadLedger(ctx context.Context, kv state.KV, m *Mapping) (*ledger, error) {
	l := &ledger{key: ledgerKey(m.Name), rows: map[string]*ledgerRow{}, held: map[string]canonical.Digest{}}
	data, err := kv.Get(ctx, l.key)
	switch {
	case errors.Is(err, fs.ErrNotExist) || os.IsNotExist(err):
		return l, nil
	case err != nil:
		return nil, fmt.Errorf("remotesync: read ledger: %w", err)
	}
	var doc ledgerDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("remotesync: the ledger %s is corrupt: %w", l.key, err)
	}
	if doc.Version != ledgerVersion {
		return nil, fmt.Errorf("remotesync: the ledger %s has version %d, want %d", l.key, doc.Version, ledgerVersion)
	}
	for _, r := range doc.Rows {
		if r == nil || (r.Kind != "E" && r.Kind != "R") || r.Address == "" {
			return nil, fmt.Errorf("remotesync: the ledger %s holds an invalid row", l.key)
		}
		switch r.State {
		case stateLinked, stateDetached, stateRejected, stateDropped:
		default:
			return nil, fmt.Errorf("remotesync: the ledger %s holds a row in state %q", l.key, r.State)
		}
		if doc.Mapping != m.Hash {
			r.Hash = canonical.Digest{}
		}
		l.rows[rowKey(r.kind(), r.Address)] = r
	}
	if doc.Held != nil && doc.Mapping == m.Hash {
		l.held = doc.Held
	}
	l.deletes = doc.Deletes
	return l, nil
}

// recentDeletes returns how many deletes the ledger holds since now minus
// [deleteWindow], and forgets the older ones.
func (l *ledger) recentDeletes(now time.Time) int {
	since := now.Add(-deleteWindow).Unix()
	l.deletes = slices.DeleteFunc(l.deletes, func(t int64) bool { return t < since })
	return len(l.deletes)
}

// save writes the ledger under m's hash.
func (l *ledger) save(ctx context.Context, kv state.KV, m *Mapping) error {
	doc := ledgerDoc{
		Version: ledgerVersion, Mapping: m.Hash, Rows: make([]*ledgerRow, 0, len(l.rows)),
		Deletes: l.deletes,
	}
	if len(l.held) > 0 {
		doc.Held = l.held
	}
	for _, k := range sortedKeys(l.rows) {
		doc.Rows = append(doc.Rows, l.rows[k])
	}
	data, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("remotesync: encode ledger: %w", err)
	}
	if err := kv.Put(ctx, l.key, data); err != nil {
		return fmt.Errorf("remotesync: write ledger: %w", err)
	}
	return nil
}

func (l *ledger) get(kind reconcile.Kind, address string) *ledgerRow {
	return l.rows[rowKey(kind, address)]
}

func (l *ledger) put(r *ledgerRow) { l.rows[rowKey(r.kind(), r.Address)] = r }

func (l *ledger) remove(r *ledgerRow) { delete(l.rows, rowKey(r.kind(), r.Address)) }

// buildIndex fixes the rows the diff sees: every row but the detached ones.
func (l *ledger) buildIndex() {
	l.index = map[reconcile.Kind][]*ledgerRow{}
	for _, r := range l.rows {
		if r.State != stateDetached {
			l.index[r.kind()] = append(l.index[r.kind()], r)
		}
	}
	for _, rows := range l.index {
		sort.Slice(rows, func(i, j int) bool { return rows[i].Address < rows[j].Address })
	}
}

// inRange returns the indexed rows of types within r, in address order.
func (l *ledger) inRange(types visibility.ElementTypes, r store.ElementRange) []*ledgerRow {
	kind, names := reconcile.KindEntity, types.Entities
	if r.Relations {
		kind, names = reconcile.KindRelation, types.Relations
	}
	rows := l.index[kind]
	start := sort.Search(len(rows), func(i int) bool { return rows[i].Address >= r.Lo })
	var out []*ledgerRow
	for _, row := range rows[start:] {
		if !r.Unbounded && row.Address >= r.Hi {
			break
		}
		if slices.Contains(names, row.Type) {
			out = append(out, row)
		}
	}
	return out
}

// local is the ledger as the local side of a diff.
type local struct{ l *ledger }

func (s local) Sum(_ context.Context, types visibility.ElementTypes, r store.ElementRange) (reconclient.Sum, error) {
	var rg reconcile.Range
	for _, row := range s.l.inRange(types, r) {
		rg = rg.Add(reconcile.Element{Kind: row.kind(), Address: row.Address, Hash: row.Hash})
	}
	return reconclient.Sum{Count: rg.Count, Fingerprint: rg.Sum.String()}, nil
}

func (s local) List(
	_ context.Context, types visibility.ElementTypes, r store.ElementRange, limit int,
) (reconclient.Page, error) {
	rows := s.l.inRange(types, r)
	var page reconclient.Page
	if limit > 0 && len(rows) > limit {
		rows, page.More = rows[:limit], true
	}
	page.Rows = make([]reconclient.Row, len(rows))
	for i, row := range rows {
		page.Rows[i] = reconclient.Row{Address: row.Address, Hash: row.Hash}
	}
	if len(rows) > 0 {
		page.Last = rows[len(rows)-1].Address
	}
	return page, nil
}

// splitRelationAddress parses "from--type--to" without a face.
func splitRelationAddress(a string) (from, typ, to string, ok bool) {
	parts := strings.Split(a, "--")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], true
}
