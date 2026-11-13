package remotesync

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/reconclient"
	"github.com/Sourcehaven-BV/rela/internal/state"
)

// Link settles the uncertain create of the local entity t.Local on m's
// conflict list by linking it to the remote row at address, which a person
// found on the remote. It is the other way out of an uncertain create:
// resolving it to the local side sends the row again, which makes a second
// copy when the first create did reach the remote.
//
// Link writes neither side. It records the pair in the ledger as a pushed
// row: adopted, so on_gone never deletes it, with the values the create
// tried to send as the merge base and no view hash, so the next sync
// compares the two rows field by field and sets the local ref. If the
// remote does not list the address, that sync detaches the row.
//
// address is a remote entity address, type/id, of the remote type the
// local type maps from. A row the ledger already holds, on either side and
// in any state, is refused: a detached or dropped pair is the sync's record
// of that row, and a link does not overwrite it. The caller holds the remote's run lock.
func Link(ctx context.Context, kv state.KV, m *Mapping, t Target, address string) error {
	if m == nil {
		return errors.New("remotesync: a mapping is required")
	}
	l, err := loadLedger(ctx, kv, m)
	if err != nil {
		return err
	}
	c, err := loadConflicts(ctx, kv, m.Name)
	if err != nil {
		return err
	}
	if err := CheckLink(c.list(), t); err != nil {
		return err
	}
	local := t.Local
	e := c.entries[conflictKey(local, "")]
	em := entityMappingOf(m, e.Type)
	if em == nil {
		return fmt.Errorf("%w: %s: the remote no longer maps type %s", ErrRefused, local, e.Type)
	}
	if err := checkLinkAddress(em, address); err != nil {
		return err
	}
	if l.get(reconclient.KindEntity, address) != nil {
		return fmt.Errorf("%w: the ledger already holds %s", ErrRefused, address)
	}
	for _, r := range l.rows {
		if r.Kind == "E" && r.Local == local {
			return fmt.Errorf("%w: the ledger already holds %s, paired with %s", ErrRefused, local, r.Address)
		}
	}
	l.put(&ledgerRow{
		Kind: "E", Type: em.Source, Address: address, Local: local, State: stateLinked,
		Base: maps.Clone(e.RefusedFields), Adopted: true,
	})
	c.drop(local, "")
	if err := l.save(ctx, kv, m); err != nil {
		return err
	}
	return c.save(ctx, kv)
}

// CheckLink reports whether list holds an uncertain create of the local
// entity t.Local that [Link] can settle, as t checked it. It fails with
// [ErrNoConflict] when there is none. Field and Exact play no part: a link
// always names the row.
func CheckLink(list []Conflict, t Target) error {
	for _, e := range list {
		if e.Local == t.Local && e.Field == "" && e.Kind == "" && e.Reason == ReasonUncertain {
			return t.checked(&e)
		}
	}
	return fmt.Errorf("%w: %s has no uncertain create to link", ErrNoConflict, t.Local)
}

// entityMappingOf returns the mapping of the local entity type local, or
// nil.
func entityMappingOf(m *Mapping, local string) *EntityMapping {
	for _, em := range m.Entities {
		if em.Local == local {
			return em
		}
	}
	return nil
}

// checkLinkAddress checks that address names a row of em's remote type:
// type/id, with an id the write API accepts.
func checkLinkAddress(em *EntityMapping, address string) error {
	typ, id, ok := strings.Cut(address, "/")
	switch {
	case !ok || typ != em.Source:
		return fmt.Errorf("%w: %q is not a remote %s address; use %s/<id>", ErrRefused, address, em.Source,
			em.Source)
	case entity.ValidateID(id) != nil:
		return fmt.Errorf("%w: %q does not name a valid id", ErrRefused, address)
	}
	return nil
}

// Linked is one row the ledger of a remote links: an entity, or a relation
// when Kind is [KindRelation]. Type is the local type, Local the local
// entity id or relation key, and Face the local entity's face.
type Linked struct {
	Kind  string
	Type  string
	Local string
	Face  entity.Face
}

// LoadLinked returns the rows the ledger of m links now, for a status page
// that counts them. Like [LoadConflicts] it is not filtered by acl.yaml: a
// caller that shows a count to anyone but an operator gates each row first.
func LoadLinked(ctx context.Context, kv state.KV, m *Mapping) ([]Linked, error) {
	if m == nil {
		return nil, errors.New("remotesync: a mapping is required")
	}
	l, err := loadLedger(ctx, kv, m)
	if err != nil {
		return nil, err
	}
	out := make([]Linked, 0, len(l.rows))
	for _, r := range l.rows {
		if r.State != stateLinked {
			continue
		}
		switch r.Kind {
		case "E":
			if em := m.Entities[r.Type]; em != nil {
				out = append(out, Linked{Type: em.Local, Local: r.Local, Face: r.Face})
			}
		case "R":
			if rm := m.Relations[r.Type]; rm != nil {
				out = append(out, Linked{Kind: KindRelation, Type: rm.Local, Local: r.Local})
			}
		}
	}
	slices.SortFunc(out, func(a, b Linked) int {
		return cmp.Or(cmp.Compare(a.Kind, b.Kind), cmp.Compare(a.Type, b.Type), cmp.Compare(a.Local, b.Local))
	})
	return out, nil
}
