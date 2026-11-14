package records

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"math/big"
	"sort"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// Owner is an owner entity a caller may append to.
type Owner struct {
	ID   string
	Type string
	// Faces holds the raw stored properties of each face of the owner, for
	// write checks such as a ledger's closed period. Rows belong to the
	// whole family, so a check holds when it holds on every face. They
	// never reach a response.
	Faces []map[string]any
}

// Owners is the access gate on owner entities. Every method resolves the
// principal on ctx per call, so one Service serves every caller, and fails
// closed without one.
type Owners interface {
	// Readable reports the ids in ids the caller may read every stored face
	// of, in one batch read. Rows belong to the whole family, so an owner
	// with a hidden face is unreadable, as it is unwritable. A row's owner
	// type was checked when it was appended, so a read needs only the row
	// gate.
	Readable(ctx context.Context, ids []string) (map[string]bool, error)
	// Writable returns the owner the caller may update. A miss (no such
	// entity, or one the caller may not read) is [ErrNotFound]; an owner
	// the caller reads but may not update is [ErrForbidden].
	Writable(ctx context.Context, id string) (Owner, error)
}

// WriteLock runs the owner check and the insert of one append as a unit
// that entity writes cannot interleave with. A wiring that keeps the rows in
// the entity store's database implements it with one store transaction, so
// an owner whose period is closed by an entity update either commits first,
// and the check sees the new lock date, or waits until the append has
// committed.
type WriteLock interface {
	// Locked runs fn inside the unit. fn reads owners on the ctx it
	// receives and writes rows through st, which is valid only during fn.
	// An error from fn leaves no rows written.
	Locked(ctx context.Context, fn func(ctx context.Context, st Store) error) error
}

// Deps are the collaborators of a [Service]. Store, Owners and Meta are
// required.
type Deps struct {
	Store  Store
	Owners Owners
	Meta   *metamodel.Metamodel
	// Lock orders appends against entity writes.
	//
	// Nil: accepted. The owner check and the insert then run one after the
	// other on Store, so an entity write that lands between them is not
	// seen. The file backend of the fs and memory tiers has no transaction
	// in common with the entity store and is wired this way.
	Lock WriteLock
	// Now is the clock. Nil means time.Now.
	Now func() time.Time
}

// Service checks access, field values and the ledger rules on top of a
// [Store]. It is safe for concurrent use.
//
// # Access
//
// A row is readable when its owner is: the owner's row gate decides, and a
// hidden owner is indistinguishable from one without rows. Appending needs
// update access on the owner. An aggregate gates its owner set first and
// only then folds, so no hidden owner's value reaches a result; results are
// per caller and never cached.
type Service struct {
	store  Store
	owners Owners
	meta   *metamodel.Metamodel
	lock   WriteLock
	now    func() time.Time
}

// NewService returns a service over d.
//
// Nil: Store, Owners and Meta are rejected.
func NewService(d Deps) (*Service, error) {
	if d.Store == nil || d.Owners == nil || d.Meta == nil {
		return nil, errors.New("records: NewService requires Store, Owners and Meta")
	}
	if d.Now == nil {
		d.Now = time.Now
	}
	return &Service{store: d.Store, owners: d.Owners, meta: d.Meta, lock: d.Lock, now: d.Now}, nil
}

// locked runs fn under the write lock, or on the store directly when none is
// wired.
func (s *Service) locked(ctx context.Context, fn func(ctx context.Context, st Store) error) error {
	if s.lock == nil {
		return fn(ctx, s.store)
	}
	return s.lock.Locked(ctx, fn)
}

// TypeInfo describes one record type for a client.
type TypeInfo struct {
	Key         string               `json:"key"`
	Label       string               `json:"label"`
	Description string               `json:"description,omitempty"`
	Owner       string               `json:"owner"`
	Fields      map[string]FieldInfo `json:"fields"`
	FieldOrder  []string             `json:"field_order"`
	Retention   string               `json:"retention,omitempty"`
	Ledger      *metamodel.LedgerDef `json:"ledger,omitempty"`
}

// FieldInfo describes one field of a record type for a client.
type FieldInfo struct {
	Type        string   `json:"type"`
	Label       string   `json:"label,omitempty"`
	Description string   `json:"description,omitempty"`
	Required    bool     `json:"required,omitempty"`
	Scale       *int     `json:"scale,omitempty"`
	Values      []string `json:"values,omitempty"`
}

// Types returns the declared record types, in key order. The schema is not a
// secret, so this is not gated.
func (s *Service) Types() []TypeInfo {
	out := make([]TypeInfo, 0, len(s.meta.Records))
	for _, k := range s.meta.RecordKeys() {
		out = append(out, typeInfo(k, s.meta.Records[k]))
	}
	return out
}

// TypesFor returns the record types owned by entity type ownerType.
func (s *Service) TypesFor(ownerType string) []TypeInfo {
	var out []TypeInfo
	for _, t := range s.Types() {
		if t.Owner == ownerType {
			out = append(out, t)
		}
	}
	return out
}

func typeInfo(key string, d metamodel.RecordTypeDef) TypeInfo {
	label := d.Label
	if label == "" {
		label = key
	}
	fields := make(map[string]FieldInfo, len(d.Fields))
	for name, p := range d.Fields {
		fields[name] = FieldInfo{Type: p.Type, Label: p.Label, Description: p.Description,
			Required: p.Required, Scale: p.Scale, Values: p.Values}
	}
	return TypeInfo{Key: key, Label: label, Description: d.Description, Owner: d.Owner, Fields: fields,
		FieldOrder: d.FieldNames(), Retention: d.Retention, Ledger: d.Ledger}
}

func (s *Service) def(typ string) (metamodel.RecordTypeDef, error) {
	d, ok := s.meta.Records[typ]
	if !ok {
		return metamodel.RecordTypeDef{}, fmt.Errorf("%w: record type %q", ErrNotFound, typ)
	}
	return d, nil
}

// Input is one row to append.
type Input struct {
	Owner string
	// TS is when the row applies. Zero means now.
	TS     time.Time
	Fields map[string]any
}

// Append appends rows of type typ atomically and returns them as stored.
// Every owner must be of the type's owner type and updatable by the caller.
// For a ledger type the rows are one or more whole transactions: each group
// must be new and sum to zero.
func (s *Service) Append(ctx context.Context, typ string, in []Input) ([]Record, error) {
	d, err := s.def(typ)
	if err != nil {
		return nil, err
	}
	if len(in) == 0 {
		return nil, fmt.Errorf("%w: no rows", ErrInvalid)
	}
	if len(in) > MaxBatch {
		return nil, fmt.Errorf("%w: at most %d rows per append", ErrInvalid, MaxBatch)
	}
	var out []Record
	err = s.locked(ctx, func(ctx context.Context, st Store) error {
		var lockedErr error
		out, lockedErr = s.appendLocked(ctx, st, typ, d, in)
		return lockedErr
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// appendLocked checks the owners and the rows and stores them on st. It runs
// under the write lock, so the owner check reads what the insert commits
// against.
func (s *Service) appendLocked(
	ctx context.Context, st Store, typ string, d metamodel.RecordTypeDef, in []Input,
) ([]Record, error) {
	owners, err := s.writableOwners(ctx, d, in)
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	who := principal.From(ctx).User
	recs := make([]Record, len(in))
	for i, r := range in {
		fields, err := normalizeFields(s.meta, d, r.Fields)
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", i+1, err)
		}
		ts := r.TS
		if ts.IsZero() {
			ts = now
		}
		recs[i] = Record{ID: newID(now, i), Type: typ, Owner: r.Owner, TS: FromMicros(Micros(ts)),
			RecordedAt: FromMicros(Micros(now)), RecordedBy: who, Fields: fields}
	}
	req := AppendRequest{Records: recs}
	if d.Ledger != nil {
		groups, err := ledgerGroups(typ, d, recs, owners)
		if err != nil {
			return nil, err
		}
		req.Groups = groups
	}
	if err := st.Append(ctx, req); err != nil {
		return nil, err
	}
	return recs, nil
}

// writableOwners checks every owner of in once.
func (s *Service) writableOwners(ctx context.Context, d metamodel.RecordTypeDef, in []Input) (map[string]Owner, error) {
	out := map[string]Owner{}
	for i, r := range in {
		if _, seen := out[r.Owner]; seen {
			continue
		}
		if r.Owner == "" {
			return nil, fmt.Errorf("row %d: %w: owner is required", i+1, ErrInvalid)
		}
		o, err := s.owners.Writable(ctx, r.Owner)
		if err != nil {
			return nil, err
		}
		if o.Type != d.Owner {
			// A readable entity of another type is not a valid owner. Say
			// so plainly: the caller can read it, so its type is no secret.
			return nil, fmt.Errorf("row %d: %w: %s is of type %s, not %s", i+1, ErrInvalid, r.Owner, o.Type, d.Owner)
		}
		out[r.Owner] = o
	}
	return out, nil
}

// ledgerGroups sets each row's group, checks the balance and then the closed
// periods, and returns the groups the append claims. The legs of one
// transaction share one ts, so no row of it can expire or close apart from
// the others.
func ledgerGroups(typ string, d metamodel.RecordTypeDef, recs []Record, owners map[string]Owner) ([]GroupKey, error) {
	l := d.Ledger
	dated := map[string]time.Time{}
	for i := range recs {
		g, _ := recs[i].Fields[l.GroupBy].(string)
		if g == "" || strings.HasSuffix(g, ReversalSuffix) {
			return nil, fmt.Errorf("row %d: %w: %s must be set and must not end in %q",
				i+1, ErrInvalid, l.GroupBy, ReversalSuffix)
		}
		recs[i].Group = g
		if ts, seen := dated[g]; seen && !ts.Equal(recs[i].TS) {
			return nil, fmt.Errorf("row %d: %w: every row of transaction %s must have the same ts", i+1, ErrInvalid, g)
		}
		dated[g] = recs[i].TS
	}
	// The closed period is checked last. Every earlier answer depends only on
	// what the caller sent, so a caller who cannot see the lock date can
	// only learn it by committing a real, balanced posting. Checked first, an
	// unbalanced probe would answer "closed" or "unbalanced" and reveal the
	// date without writing anything.
	if err := checkBalance(l, recs); err != nil {
		return nil, err
	}
	for i := range recs {
		if err := checkOpen(l, recs[i], owners[recs[i].Owner]); err != nil {
			return nil, fmt.Errorf("row %d: %w", i+1, err)
		}
	}
	groups := make([]GroupKey, 0, len(dated))
	for g := range dated {
		groups = append(groups, GroupKey{Type: typ, Group: g})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Group < groups[j].Group })
	return groups, nil
}

// checkBalance refuses rows whose amounts do not sum to zero per group and
// commodity. Every row must have its Group set.
func checkBalance(l *metamodel.LedgerDef, recs []Record) error {
	sums := map[[2]string]*big.Rat{}
	keys := make([][2]string, 0)
	for i, r := range recs {
		v, _, err := metamodel.ParseDecimal(r.Fields[l.Amount])
		if err != nil {
			return fmt.Errorf("row %d: %w: %s is not a number", i+1, ErrInvalid, l.Amount)
		}
		commodity := ""
		if l.Commodity != "" {
			c, ok := r.Fields[l.Commodity].(string)
			if !ok || c == "" {
				return fmt.Errorf("row %d: %w: %s is required", i+1, ErrInvalid, l.Commodity)
			}
			commodity = c
		}
		k := [2]string{r.Group, commodity}
		if sums[k] == nil {
			sums[k] = new(big.Rat)
			keys = append(keys, k)
		}
		sums[k].Add(sums[k], v)
	}
	for _, k := range keys {
		if sums[k].Sign() == 0 {
			continue
		}
		label := k[0]
		if k[1] != "" {
			label += " (" + k[1] + ")"
		}
		return fmt.Errorf("%w: %w: %s is off by %s", ErrInvalid, ErrUnbalanced, label, sums[k].RatString())
	}
	return nil
}

// checkOpen refuses a row dated on or before the closed_before date of any
// face of its owner.
func checkOpen(l *metamodel.LedgerDef, rec Record, o Owner) error {
	if l.ClosedBefore == "" {
		return nil
	}
	for _, props := range o.Faces {
		if err := checkOpenFace(l, rec, o.ID, props[l.ClosedBefore]); err != nil {
			return err
		}
	}
	return nil
}

func checkOpenFace(l *metamodel.LedgerDef, rec Record, id string, raw any) error {
	if raw == nil {
		return nil
	}
	var lock time.Time
	switch v := raw.(type) {
	case time.Time:
		lock = v
	case string:
		t, err := time.Parse(time.DateOnly, v)
		if err != nil {
			// An unreadable lock date fails closed: a row the owner may
			// have closed is not admitted on a guess.
			return fmt.Errorf("%w: %w: %s has an unreadable %s", ErrInvalid, ErrClosed, id, l.ClosedBefore)
		}
		lock = t
	default:
		return fmt.Errorf("%w: %w: %s has an unreadable %s", ErrInvalid, ErrClosed, id, l.ClosedBefore)
	}
	day := time.Date(rec.TS.Year(), rec.TS.Month(), rec.TS.Day(), 0, 0, 0, 0, time.UTC)
	if !day.After(time.Date(lock.Year(), lock.Month(), lock.Day(), 0, 0, 0, 0, time.UTC)) {
		// The lock date is not named: the caller may not be allowed to see
		// the property that holds it.
		return fmt.Errorf("%w: %w: the period is closed for %s", ErrInvalid, ErrClosed, id)
	}
	return nil
}

// Reverse appends the negation of ledger transaction group, dated now, and
// returns the new rows. The caller must be able to read every owner in the
// transaction, or it is [ErrNotFound], and to update every owner, or it is
// [ErrForbidden]. A transaction is reversed at most once.
func (s *Service) Reverse(ctx context.Context, typ, group string) ([]Record, error) {
	d, err := s.def(typ)
	if err != nil {
		return nil, err
	}
	if d.Ledger == nil {
		return nil, fmt.Errorf("%w: %s is not a ledger", ErrInvalid, typ)
	}
	if group == "" {
		return nil, fmt.Errorf("%w: group is required", ErrInvalid)
	}
	var out []Record
	err = s.locked(ctx, func(ctx context.Context, st Store) error {
		var lockedErr error
		out, lockedErr = s.reverseLocked(ctx, st, typ, d, group)
		return lockedErr
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// reverseLocked reads the transaction, checks its owners and stores the
// reversal on st, under the write lock as appendLocked does.
func (s *Service) reverseLocked(
	ctx context.Context, st Store, typ string, d metamodel.RecordTypeDef, group string,
) ([]Record, error) {
	orig, err := groupRows(ctx, st, typ, group)
	if err != nil {
		return nil, err
	}
	// A transaction with a leg the caller cannot read is not found, as in
	// [Service.Transaction], whichever owner the update check meets first.
	if readErr := s.everyOwnerReadable(ctx, group, orig); readErr != nil {
		return nil, readErr
	}
	in := make([]Input, len(orig))
	for i, r := range orig {
		in[i] = Input{Owner: r.Owner, Fields: r.Fields}
	}
	owners, err := s.writableOwners(ctx, d, in)
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	who := principal.From(ctx).User
	rev := group + ReversalSuffix
	recs := make([]Record, len(orig))
	for i, r := range orig {
		fields := maps.Clone(r.Fields)
		fields[d.Ledger.GroupBy] = rev
		amount, _, err := metamodel.ParseDecimal(r.Fields[d.Ledger.Amount])
		if err != nil {
			return nil, fmt.Errorf("%w: stored row %s has no amount", ErrInvalid, r.ID)
		}
		neg := new(big.Rat).Neg(amount)
		if d.Fields[d.Ledger.Amount].Type == metamodel.PropertyTypeInteger {
			if !neg.Num().IsInt64() {
				return nil, fmt.Errorf("%w: row %s cannot be negated", ErrInvalid, r.ID)
			}
			fields[d.Ledger.Amount] = neg.Num().Int64()
		} else {
			fields[d.Ledger.Amount] = neg.FloatString(decimalPlaces(r.Fields[d.Ledger.Amount]))
		}
		recs[i] = Record{ID: newID(now, i), Type: typ, Owner: r.Owner, TS: FromMicros(Micros(now)),
			RecordedAt: FromMicros(Micros(now)), RecordedBy: who, Group: rev, Reverses: group, Fields: fields}
		if err := checkOpen(d.Ledger, recs[i], owners[r.Owner]); err != nil {
			return nil, err
		}
	}
	// The stored legs balanced when they were appended; check again so a
	// transaction that lost a leg is never reversed into an unbalanced one.
	if err := checkBalance(d.Ledger, recs); err != nil {
		return nil, err
	}
	req := AppendRequest{Records: recs, Groups: []GroupKey{{Type: typ, Group: rev}}}
	if err := st.Append(ctx, req); err != nil {
		return nil, err
	}
	return recs, nil
}

// groupRows returns every row of group, across owners, unfiltered. Callers
// gate the result. An append holds at most [MaxBatch] rows, so a longer
// group is refused rather than read without bound.
func groupRows(ctx context.Context, st Store, typ, group string) ([]Record, error) {
	if group == "" {
		return nil, fmt.Errorf("%w: group is required", ErrInvalid)
	}
	var out []Record
	q := ListQuery{Type: typ, Group: group, Limit: MaxPage}
	for {
		page, err := st.List(ctx, q)
		if err != nil {
			return nil, err
		}
		out = append(out, page.Records...)
		if len(out) > MaxBatch {
			return nil, fmt.Errorf("%w: transaction %q has more than %d rows", ErrInvalid, group, MaxBatch)
		}
		if page.Next == "" {
			return out, nil
		}
		q.After = page.Next
	}
}

// Transaction returns the rows of ledger transaction group. The caller must
// be able to read every owner in it; otherwise it is [ErrNotFound], so the
// visible legs never reveal the sum of hidden ones.
func (s *Service) Transaction(ctx context.Context, typ, group string) ([]Record, error) {
	d, err := s.def(typ)
	if err != nil {
		return nil, err
	}
	if d.Ledger == nil {
		return nil, fmt.Errorf("%w: %s is not a ledger", ErrInvalid, typ)
	}
	rows, err := groupRows(ctx, s.store, typ, group)
	if err != nil {
		return nil, err
	}
	if err := s.everyOwnerReadable(ctx, group, rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// everyOwnerReadable returns [ErrNotFound] unless rows is not empty and the
// caller may read the owner of every row.
func (s *Service) everyOwnerReadable(ctx context.Context, group string, rows []Record) error {
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.Owner)
	}
	readable, err := s.owners.Readable(ctx, ids)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return fmt.Errorf("%w: transaction %q", ErrNotFound, group)
	}
	for _, r := range rows {
		if !readable[r.Owner] {
			return fmt.Errorf("%w: transaction %q", ErrNotFound, group)
		}
	}
	return nil
}

// List returns one page of owner's rows of type typ.
func (s *Service) List(ctx context.Context, typ, owner string, q ListQuery) (Page, error) {
	if _, err := s.def(typ); err != nil {
		return Page{}, err
	}
	if err := s.readableOwner(ctx, owner); err != nil {
		return Page{}, err
	}
	q.Type, q.Owner, q.Group = typ, owner, ""
	page, err := s.store.List(ctx, q)
	if page.Records == nil {
		page.Records = []Record{}
	}
	return page, err
}

func (s *Service) readableOwner(ctx context.Context, owner string) error {
	if owner == "" {
		return fmt.Errorf("%w: owner is required", ErrInvalid)
	}
	got, err := s.owners.Readable(ctx, []string{owner})
	if err != nil {
		return err
	}
	if !got[owner] {
		return fmt.Errorf("%w: %s", ErrNotFound, owner)
	}
	return nil
}

// Aggregate folds the rows of q.Type over the owners the caller may read.
// With no owners in q it folds every readable owner with rows. Owners are
// gated before anything folds.
func (s *Service) Aggregate(ctx context.Context, q AggregateQuery) ([]Point, error) {
	d, err := s.def(q.Type)
	if err != nil {
		return nil, err
	}
	scale := 0
	if q.Field != "" {
		p, ok := d.Fields[q.Field]
		if !ok || !metamodel.IsNumericType(p.Type) {
			return nil, fmt.Errorf("%w: %s is not a numeric field of %s", ErrInvalid, q.Field, q.Type)
		}
		scale = scaleOf(p)
	}
	owners := q.Owners
	if len(owners) == 0 {
		if owners, err = s.store.Owners(ctx, q.Type); err != nil {
			return nil, err
		}
	}
	readable, err := s.owners.Readable(ctx, owners)
	if err != nil {
		return nil, err
	}
	gated := make([]string, 0, len(owners))
	seen := map[string]bool{}
	for _, o := range owners {
		if readable[o] && !seen[o] {
			seen[o] = true
			gated = append(gated, o)
		}
	}
	if len(q.Owners) == 1 && len(gated) == 0 {
		// One named owner that is hidden: not found, as a list would say.
		return nil, fmt.Errorf("%w: %s", ErrNotFound, q.Owners[0])
	}
	q.Owners = gated
	if len(gated) == 0 {
		return []Point{}, nil
	}
	if a, ok := s.store.(Aggregator); ok {
		return a.Aggregate(ctx, q, scale)
	}
	return AggregateGeneric(ctx, s.store, q, scale)
}

// Expire drops the rows older than each type's retention and returns how
// many it dropped.
func (s *Service) Expire(ctx context.Context) (int64, error) {
	var total int64
	now := s.now()
	for _, k := range s.meta.RecordKeys() {
		r := s.meta.Records[k].RetentionDuration()
		if r <= 0 {
			continue
		}
		n, err := s.store.Expire(ctx, k, now.Add(-r))
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func scaleOf(p metamodel.PropertyDef) int {
	if p.Type != metamodel.PropertyTypeDecimal {
		return 0
	}
	if p.Scale != nil {
		return *p.Scale
	}
	return -1
}

// decimalPlaces returns the digits after the point of a stored decimal.
func decimalPlaces(v any) int {
	s, _ := v.(string)
	if _, frac, ok := strings.Cut(s, "."); ok {
		return len(frac)
	}
	return 0
}

// newID returns a row id that sorts by creation and is unique: the time in
// microseconds, the row's place in its batch, and 8 random bytes, in hex.
func newID(now time.Time, seq int) string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%014x%04x%s", now.UnixMicro(), seq, hex.EncodeToString(b[:]))
}
