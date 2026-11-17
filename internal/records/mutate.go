package records

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// Change is an update of one row of a mutable type (TKT-ERO2X5G).
type Change struct {
	// Revision is the revision of the row the caller read. A row changed
	// since then is [ErrStale].
	Revision int64
	// TS moves the row to another time. Zero keeps it.
	TS time.Time
	// Fields are the fields to change. A nil value clears an optional
	// field. A field not named keeps its value.
	Fields map[string]any
}

// Update changes row id of mutable type typ and returns it as stored. The
// caller must be able to update the row's owner; the owner of a row does not
// change. The change is audited.
func (s *Service) Update(ctx context.Context, typ, id string, c Change) (Record, error) {
	d, err := s.mutableDef(typ)
	if err != nil {
		return Record{}, err
	}
	if len(c.Fields) == 0 && c.TS.IsZero() {
		return Record{}, fmt.Errorf("%w: a change names no field and no ts", ErrInvalid)
	}
	var out Record
	now := s.now().UTC()
	err = s.locked(ctx, func(ctx context.Context, st Store) error {
		m, cur, cerr := s.changeable(ctx, st, typ, id, c.Revision)
		if cerr != nil {
			return cerr
		}
		// A stored field the schema no longer declares is dropped rather
		// than refused, so a schema change never locks old rows.
		fields := maps.Clone(cur.Fields)
		maps.DeleteFunc(fields, func(k string, _ any) bool { _, ok := d.Fields[k]; return !ok })
		maps.Copy(fields, c.Fields)
		next := cur
		var nerr error
		if next.Fields, nerr = normalizeFields(s.meta, d, fields); nerr != nil {
			return nerr
		}
		if !c.TS.IsZero() {
			next.TS = FromMicros(Micros(c.TS))
		}
		next.UpdatedAt = FromMicros(Micros(now))
		next.UpdatedBy = principal.From(ctx).User
		next.Revision = cur.Revision + 1
		if rerr := m.Replace(ctx, next); rerr != nil {
			return rerr
		}
		out = next
		return nil
	})
	if err != nil {
		s.denied(ctx, err, audit.OpUpdateRecord, typ, id)
		return Record{}, err
	}
	s.record(ctx, audit.OpUpdateRecord, out, fmt.Sprintf("updated row of %s to revision %d; fields: %s",
		out.Owner, out.Revision, strings.Join(slices.Sorted(maps.Keys(c.Fields)), ", ")))
	return out, nil
}

// Delete removes row id of mutable type typ. The caller must be able to
// update the row's owner and name the revision it read. The delete is
// audited.
func (s *Service) Delete(ctx context.Context, typ, id string, revision int64) error {
	if _, err := s.mutableDef(typ); err != nil {
		return err
	}
	var gone Record
	err := s.locked(ctx, func(ctx context.Context, st Store) error {
		m, cur, err := s.changeable(ctx, st, typ, id, revision)
		if err != nil {
			return err
		}
		if err := m.Delete(ctx, typ, id, revision); err != nil {
			return err
		}
		gone = cur
		return nil
	})
	if err != nil {
		s.denied(ctx, err, audit.OpDeleteRecord, typ, id)
		return err
	}
	s.record(ctx, audit.OpDeleteRecord, gone, fmt.Sprintf("deleted row of %s at revision %d",
		gone.Owner, gone.Revision))
	return nil
}

// mutableDef returns the definition of typ, which must be mutable. Which
// types are mutable is schema, not a secret, so this is checked first.
func (s *Service) mutableDef(typ string) (metamodel.RecordTypeDef, error) {
	d, err := s.def(typ)
	if err != nil {
		return d, err
	}
	if !d.Mutable {
		return d, fmt.Errorf("%w: rows of %s cannot be changed; the type is not mutable", ErrInvalid, typ)
	}
	return d, nil
}

// changeable reads row id on st and checks that the caller may change it:
// a row whose owner the caller cannot read is [ErrNotFound], like a row that
// does not exist, and one whose owner the caller cannot update is
// [ErrForbidden]. Only then is the revision compared, so a stale revision
// tells nothing about a hidden row.
func (s *Service) changeable(
	ctx context.Context, st Store, typ, id string, revision int64,
) (Mutator, Record, error) {
	m, ok := st.(Mutator)
	if !ok {
		// NewService refuses a mutable type over a store without the
		// capability; a lock that hands out another store is a wiring bug.
		return nil, Record{}, fmt.Errorf("records: the record store cannot change rows of %s", typ)
	}
	if id == "" {
		return nil, Record{}, fmt.Errorf("%w: record id is required", ErrInvalid)
	}
	cur, err := m.Get(ctx, typ, id)
	if err != nil {
		return nil, Record{}, err
	}
	if err := s.readableOwner(ctx, cur.Owner); err != nil {
		if errors.Is(err, ErrNotFound) {
			// Name the row, not its hidden owner.
			err = fmt.Errorf("%w: record %s", ErrNotFound, id)
		}
		return nil, Record{}, err
	}
	if _, err := s.owners.Writable(ctx, cur.Owner); err != nil {
		return nil, Record{}, err
	}
	if cur.Revision != revision {
		return nil, Record{}, fmt.Errorf("%w: record %s is at revision %d", ErrStale, id, cur.Revision)
	}
	return m, cur, nil
}

// record writes the audit line of a change to rec. It runs after the change
// has committed, so a rolled-back change leaves no line. The line names the
// changed fields, not their values: the row holds only its latest values.
func (s *Service) record(ctx context.Context, op string, rec Record, summary string) {
	s.write(ctx, op, rec.Type, rec.ID, summary)
}

// denied writes a denied-write line when err is [ErrForbidden]: the caller
// can read the owner but not update it. A [ErrNotFound] gets no line, so the
// log never shows that a hidden row exists.
func (s *Service) denied(ctx context.Context, err error, op, typ, id string) {
	if errors.Is(err, ErrForbidden) {
		s.write(ctx, audit.OpDeniedWrite, typ, id, "denied: may not update the owner (op="+op+")")
	}
}

func (s *Service) write(ctx context.Context, op, typ, id, summary string) {
	s.audit.Record(audit.Stamp(ctx, audit.Record{
		Time:        s.now().UTC(),
		Op:          op,
		Subject:     &audit.Subject{Kind: "record", Type: typ, ID: id},
		Principal:   principal.From(ctx),
		TriggeredBy: audit.TriggeredByFrom(ctx),
		AssertedBy:  audit.AssertedByFrom(ctx),
		Summary:     summary,
	}))
}
