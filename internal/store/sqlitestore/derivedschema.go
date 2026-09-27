package sqlitestore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Derived indexes (TKT-B51CYD) are expression indexes synthesized from the
// validated static queries in data-entry.yaml, as pgstore derives them: a
// query index serves a static query's scalar equality filters, a list index
// serves a list page's equality filters plus its sort. Their names carry an
// owned prefix, and only objects under that prefix are ever dropped.
//
// Each index is written with the SAME helpers the query builder uses
// (rawExpr, typeExpr, orderKeySQL), with the alias left empty. SQLite uses an
// expression index only when the query's expression is structurally the
// indexed one, so a second spelling here would be correct and silently
// unindexed.
//
// DerivedUnique specs are not handled: `unique:` stays an application-level
// check on this single-writer backend.

const (
	derivedQueryPrefix = "rela_derived_query__"
	derivedListPrefix  = "rela_derived_list__"
)

// Reconcile converges the owned derived indexes on desired, the complete
// desired set. An owned index that is not desired is DROPPED, so a caller
// that could not load the whole set must not call this at all.
//
// It runs in one BEGIN IMMEDIATE transaction under the store's write mutex,
// so a concurrent writer waits rather than seeing half an index set; with
// opts.DryRun the transaction is rolled back and the outcomes report what
// would change. An index whose stored definition differs from the desired one
// (the builder's spelling changed) is dropped and recreated.
//
// It does not go through [Store.Tx]: a dry run must roll back a transaction
// that succeeded, and DDL is not a store write, so it takes no events.
// Called on a Tx view it refuses, because the view's open transaction already
// holds the database's write lock.
func Reconcile(
	ctx context.Context, s *Store, desired []store.DerivedObjectSpec, opts store.ReconcileOptions,
) ([]store.DerivedObjectOutcome, error) {
	if s.conn != nil {
		return nil, errors.New("sqlitestore: reconcile: called inside a transaction")
	}
	wanted, outcomes := desiredIndexes(desired)

	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("sqlitestore: reconcile: acquire connection: %w", err)
	}
	defer func() { _ = conn.Close() }()
	if _, err = conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return nil, fmt.Errorf("sqlitestore: reconcile: begin: %w", err)
	}
	done := false
	defer func() {
		if !done {
			_, _ = conn.ExecContext(context.WithoutCancel(ctx), "ROLLBACK")
		}
	}()

	existing, err := ownedIndexes(ctx, conn)
	if err != nil {
		return nil, err
	}

	for _, name := range sortedKeys(existing) {
		w, ok := wanted[name]
		if ok && w.ddl == existing[name] {
			continue
		}
		reason := "index " + name + " no longer declared"
		if ok {
			reason = "index " + name + " definition changed"
		}
		if !opts.DryRun {
			if _, err := conn.ExecContext(ctx, `DROP INDEX "`+name+`"`); err != nil {
				return nil, fmt.Errorf("sqlitestore: reconcile: drop %s: %w", name, err)
			}
		}
		delete(existing, name)
		if !ok {
			outcomes = append(outcomes, store.DerivedObjectOutcome{
				Spec: store.DerivedObjectSpec{Kind: kindOf(name)}, State: store.DerivedDropped,
				Reason: reason, WouldChange: opts.DryRun,
			})
		}
	}

	for _, name := range sortedKeys(wanted) {
		w := wanted[name]
		if _, ok := existing[name]; ok {
			outcomes = append(outcomes, store.DerivedObjectOutcome{Spec: w.spec, State: store.DerivedEnforced})
			continue
		}
		if opts.DryRun {
			outcomes = append(outcomes, store.DerivedObjectOutcome{
				Spec: w.spec, State: store.DerivedCreated, WouldChange: true,
			})
			continue
		}
		if _, err := conn.ExecContext(ctx, w.ddl); err != nil {
			outcomes = append(outcomes, store.DerivedObjectOutcome{
				Spec: w.spec, State: store.DerivedUnenforced, Reason: "index could not be created: " + err.Error(),
			})
			continue
		}
		outcomes = append(outcomes, store.DerivedObjectOutcome{Spec: w.spec, State: store.DerivedCreated})
	}

	if opts.DryRun {
		return outcomes, nil // the deferred ROLLBACK discards nothing
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return nil, fmt.Errorf("sqlitestore: reconcile: commit: %w", err)
	}
	done = true
	return outcomes, nil
}

// indexWant is one desired index: the spec it serves and its exact DDL.
type indexWant struct {
	spec store.DerivedObjectSpec
	ddl  string
}

// desiredIndexes renders each spec's DDL by index name. A spec that cannot be
// rendered safely is reported unenforced; unique specs are skipped.
func desiredIndexes(desired []store.DerivedObjectSpec) (map[string]indexWant, []store.DerivedObjectOutcome) {
	wanted := map[string]indexWant{}
	var outcomes []store.DerivedObjectOutcome
	for _, spec := range desired {
		var name, ddl string
		var ok bool
		switch spec.Kind {
		case store.DerivedQueryIndex:
			spec.Properties = slices.Compact(slices.Sorted(slices.Values(spec.Properties)))
			name, ddl, ok = queryIndexDDL(spec)
		case store.DerivedListIndex:
			name, ddl, ok = listIndexDDL(spec)
		default:
			continue
		}
		if !ok {
			outcomes = append(outcomes, store.DerivedObjectOutcome{
				Spec: spec, State: store.DerivedUnenforced, Reason: "invalid or unsafe index shape",
			})
			continue
		}
		wanted[name] = indexWant{spec: spec, ddl: ddl}
	}
	return wanted, outcomes
}

// ownedIndexes lists the indexes under the owned prefixes, name to stored DDL.
func ownedIndexes(ctx context.Context, conn querier) (map[string]string, error) {
	rows, err := collectRows(ctx, conn,
		`SELECT name, sql FROM sqlite_schema WHERE type = 'index' AND (name GLOB ?1 OR name GLOB ?2)`,
		[]any{derivedQueryPrefix + "*", derivedListPrefix + "*"},
		func(sc scanner) ([2]string, error) {
			var row [2]string
			return row, sc.Scan(&row[0], &row[1])
		})
	if err != nil {
		return nil, fmt.Errorf("sqlitestore: reconcile: list indexes: %w", err)
	}
	existing := make(map[string]string, len(rows))
	for _, row := range rows {
		existing[row[0]] = row[1]
	}
	return existing, nil
}

// queryIndexDDL derives the index serving a static query's scalar equality
// filters: keyed on the type and each property's `->>`, partial on the default
// face and on each property holding a string, which is exactly what
// scalarEqualCond tests.
func queryIndexDDL(spec store.DerivedObjectSpec) (name, ddl string, ok bool) {
	if spec.Type == "" || len(spec.Properties) == 0 {
		return "", "", false
	}
	b := &sqlBuilder{}
	cols := []string{"type"}
	guards := []string{"face = ''"}
	for _, p := range spec.Properties {
		path := b.jsonPath(p)
		cols = append(cols, rawExpr("", path))
		guards = append(guards, typeExpr("", path)+" = 'text'")
	}
	name = derivedQueryPrefix + specHash(spec)
	return name, `CREATE INDEX "` + name + `" ON entities (` + strings.Join(cols, ", ") + `) WHERE ` +
		strings.Join(guards, " AND "), !b.unsafe
}

// listIndexDDL derives the index serving a list page: the type, the equality
// properties, then each sort key as orderKeySQL spells it, then the id
// tiebreak, partial on the default face.
func listIndexDDL(spec store.DerivedObjectSpec) (name, ddl string, ok bool) {
	if spec.Type == "" || len(spec.OrderBy) == 0 {
		return "", "", false
	}
	b := &sqlBuilder{}
	cols := []string{"type"}
	for _, p := range spec.Properties {
		cols = append(cols, rawExpr("", b.jsonPath(p)))
	}
	for i, p := range spec.OrderBy {
		var values []string
		if i < len(spec.OrderValues) {
			values = spec.OrderValues[i]
		}
		// orderKeySQL appends " ASC" to each key; an index column takes it too.
		cols = append(cols, orderKeySQL(b, "", store.OrderSpec{Property: p, Values: values}))
	}
	cols = append(cols, "id")
	name = derivedListPrefix + specHash(spec)
	return name, `CREATE INDEX "` + name + `" ON entities (` + strings.Join(cols, ", ") + `) WHERE face = ''`,
		!b.unsafe
}

// specHash names an index by everything that shapes it, as pgstore's index
// names do, so a changed spec gets a new name.
func specHash(spec store.DerivedObjectSpec) string {
	h := sha256.New()
	_, _ = h.Write([]byte(spec.Type))
	for _, p := range spec.Properties {
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(p))
	}
	_, _ = h.Write([]byte{1})
	for i, p := range spec.OrderBy {
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(p))
		if i < len(spec.OrderValues) {
			for _, v := range spec.OrderValues[i] {
				_, _ = h.Write([]byte{2})
				_, _ = h.Write([]byte(v))
			}
		}
	}
	return hex.EncodeToString(h.Sum(nil)[:16])
}

func kindOf(name string) store.DerivedObjectKind {
	if strings.HasPrefix(name, derivedListPrefix) {
		return store.DerivedListIndex
	}
	return store.DerivedQueryIndex
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
