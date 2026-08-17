package pgstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// unsafeDDLNameChar mirrors metamodel.unsafeSchemaNameChar. It is duplicated
// here (rather than importing metamodel — an arch-lint boundary the store layer
// must not cross) as pure defense-in-depth: the metamodel already rejects these
// names at load, and quoteLiteral escapes safely, but the reconciler must never
// interpolate a name carrying a quote, backslash, or control character into DDL
// even if it somehow arrives unvalidated.
var unsafeDDLNameChar = regexp.MustCompile(`['\\\x00-\x1f\x7f]`)

// safeDDLName reports whether name is safe to interpolate into reconciler DDL.
func safeDDLName(name string) bool {
	return name != "" &&
		!unsafeDDLNameChar.MatchString(name) &&
		strings.TrimSpace(name) == name
}

// Derived-schema reconciliation (TKT-3Q0GP1). pgstore implements the optional
// [store.DerivedSchemaReconciler] capability: it synthesizes Postgres objects
// from the metamodel that enforce declarations atomically which the application
// otherwise checks non-atomically. Today the one rule is `unique: true`, which
// becomes a partial unique EXPRESSION index over `properties->>'<prop>'` scoped
// to `type = '<type>'`.
//
// Reconciliation is STATELESS: the metamodel is the desired set, the live
// catalog (pg_indexes) is the actual set, and a name prefix marks ownership.
// See the package/interface docs on store.DerivedSchemaReconciler.

// reconcileAdvisoryLockKey serializes concurrent reconcilers OF THIS SCHEMA,
// analogous to migrateAdvisoryLockKey. "RELD" (derived). Distinct from the
// migrate/write/sweep keys so a reconcile never blocks (or is blocked by) an
// unrelated DDL/write path spuriously.
const reconcileAdvisoryLockKey = 0x52_45_4c_44 // "RELD"

// derivedUniquePrefix is the name namespace the `unique` rule OWNS. The
// reconciler only ever drops indexes under this exact prefix, so a future rule
// (e.g. an enum CHECK under a different prefix) cannot be clobbered by this one.
// It is also the discriminator the write path matches on (see mapUniqueViolation).
const derivedUniquePrefix = "rela_derived_uniq__"

// uniqueIndexName is the deterministic index name for a (type, property) unique
// rule. Deterministic across processes and versions (no per-run entropy) so the
// drop side of reconcile is safe: an index whose name is not recomputed from the
// current metamodel is, by definition, no longer desired. A NUL separator makes
// the (type, property) encoding unambiguous so ("ab","c") and ("a","bc") cannot
// collide. The SHA-256 hex is truncated to keep the whole name within Postgres's
// 63-byte identifier limit (prefix 19 + 32 hex = 51 bytes).
func uniqueIndexName(entityType, property string) string {
	sum := sha256.Sum256([]byte(entityType + "\x00" + property))
	return derivedUniquePrefix + hex.EncodeToString(sum[:16])
}

// Reconcile implements store.DerivedSchemaReconciler. It converges the derived
// schema for desired and returns one outcome per desired spec plus one per
// discovered orphan. It requires the store handle to be a *pgxpool.Pool (so it
// can hold a session-scoped advisory lock on one connection while issuing each
// DDL statement in its own implicit transaction — a failed CREATE must not
// abort the successful ones). On any other handle it degrades: it returns nil
// outcomes and no error, exactly as a store-open reconcile on an unsupported
// backend would.
func (s *Store) Reconcile(
	ctx context.Context, desired []store.DerivedObjectSpec, opts store.ReconcileOptions,
) ([]store.DerivedObjectOutcome, error) {
	pool, ok := s.db.(*pgxpool.Pool)
	if !ok {
		return nil, nil
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("pgstore: reconcile: acquire connection: %w", err)
	}
	defer conn.Release()

	// Session-scoped advisory lock: only one process reconciles this schema at a
	// time, so concurrent booting processes cannot race a create against a drop.
	// A dry-run takes it too — it reads the catalog and must see a stable actual
	// set. Released explicitly (and by connection release) below.
	if _, lockErr := conn.Exec(ctx,
		`SELECT pg_advisory_lock($1::int, hashtext(current_schema()))`,
		reconcileAdvisoryLockKey); lockErr != nil {
		return nil, fmt.Errorf("pgstore: reconcile: acquire lock: %w", lockErr)
	}
	defer func() {
		_, _ = conn.Exec(ctx,
			`SELECT pg_advisory_unlock($1::int, hashtext(current_schema()))`, reconcileAdvisoryLockKey)
	}()

	// desiredByName maps the deterministic index name -> spec, for the "unique"
	// rule. Skip any spec whose names are unsafe for DDL interpolation — this is
	// defense-in-depth over the metamodel's load-time ValidateSchemaName; a bad
	// name is reported as unenforced rather than silently interpolated.
	desiredByName := make(map[string]store.DerivedObjectSpec, len(desired))
	var outcomes []store.DerivedObjectOutcome
	for _, spec := range desired {
		if spec.Kind != store.DerivedUnique {
			continue // unknown rule kind: not handled by this reconciler yet
		}
		if !safeDDLName(spec.Type) {
			outcomes = append(outcomes, unenforced(spec, "unsafe entity type name for DDL: "+spec.Type))
			continue
		}
		if !safeDDLName(spec.Property) {
			outcomes = append(outcomes, unenforced(spec, "unsafe property name for DDL: "+spec.Property))
			continue
		}
		desiredByName[uniqueIndexName(spec.Type, spec.Property)] = spec
	}

	// actual: this schema's owned unique-rule indexes.
	actual, err := listOwnedUniqueIndexes(ctx, conn)
	if err != nil {
		return nil, fmt.Errorf("pgstore: reconcile: list indexes: %w", err)
	}

	// DROP orphans: owned indexes not in the desired set.
	for name := range actual {
		if _, want := desiredByName[name]; want {
			continue
		}
		if opts.DryRun {
			outcomes = append(outcomes, store.DerivedObjectOutcome{
				Spec:        store.DerivedObjectSpec{Kind: store.DerivedUnique},
				State:       store.DerivedDropped,
				Reason:      "index " + name + " no longer declared",
				WouldChange: true,
			})
			continue
		}
		if _, err := conn.Exec(ctx, `DROP INDEX IF EXISTS `+quoteIdent(name)); err != nil {
			return nil, fmt.Errorf("pgstore: reconcile: drop %s: %w", name, err)
		}
		outcomes = append(outcomes, store.DerivedObjectOutcome{
			Spec:   store.DerivedObjectSpec{Kind: store.DerivedUnique},
			State:  store.DerivedDropped,
			Reason: "index " + name + " no longer declared",
		})
	}

	// CREATE missing / report enforced. Deterministic order for stable output.
	for _, name := range sortedNames(desiredByName) {
		spec := desiredByName[name]
		if _, exists := actual[name]; exists {
			outcomes = append(outcomes, store.DerivedObjectOutcome{Spec: spec, State: store.DerivedEnforced})
			continue
		}
		if opts.DryRun {
			// Predict whether a create WOULD succeed by counting current
			// violators, so a dry-run matches the real store-open outcome.
			outcomes = append(outcomes, predictUniqueCreate(ctx, conn, spec, name, opts.ShowValues))
			continue
		}
		outcomes = append(outcomes, createUniqueIndex(ctx, conn, spec, name, opts.ShowValues))
	}

	return outcomes, nil
}

// listOwnedUniqueIndexes returns the set of this schema's unique-rule indexes.
// It is scoped to current_schema() so a store sharing a database with other
// schemas never sees — and so never drops — another schema's owned indexes.
func listOwnedUniqueIndexes(ctx context.Context, conn *pgxpool.Conn) (map[string]struct{}, error) {
	rows, err := conn.Query(ctx,
		`SELECT indexname FROM pg_indexes
		 WHERE schemaname = current_schema() AND indexname LIKE $1`,
		derivedUniquePrefix+`%`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]struct{})
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out[name] = struct{}{}
	}
	return out, rows.Err()
}

// createUniqueIndex issues the partial unique expression index for spec. The
// predicate is EMPTY-EXEMPT (`... <> ” AND ... IS NOT NULL`) to match the
// application scan's semantics, which skip empty/absent values — so the two
// enforcement paths agree. A failure is almost always pre-existing duplicate
// values: it is reported as unenforced (with a blocking count, and sample
// values only if the operator opted in), NEVER returned as an error, so a
// derived-schema problem cannot fail store-open.
func createUniqueIndex(
	ctx context.Context, conn *pgxpool.Conn, spec store.DerivedObjectSpec, name string, showValues bool,
) store.DerivedObjectOutcome {
	ddl := fmt.Sprintf(
		`CREATE UNIQUE INDEX IF NOT EXISTS %s ON entities (type, (properties->>%s)) `+
			`WHERE type = %s AND properties->>%s <> '' AND properties->>%s IS NOT NULL`,
		quoteIdent(name),
		quoteLiteral(spec.Property),
		quoteLiteral(spec.Type),
		quoteLiteral(spec.Property), quoteLiteral(spec.Property))

	if _, err := conn.Exec(ctx, ddl); err != nil {
		return unenforcedFromCreateErr(ctx, conn, spec, err, showValues)
	}
	return store.DerivedObjectOutcome{Spec: spec, State: store.DerivedCreated}
}

// predictUniqueCreate reports what a create WOULD do for a dry-run: enforced if
// no violators exist, unenforced (with counts) otherwise. It never issues DDL.
func predictUniqueCreate(
	ctx context.Context, conn *pgxpool.Conn, spec store.DerivedObjectSpec, _ string, showValues bool,
) store.DerivedObjectOutcome {
	count, samples, err := uniqueViolators(ctx, conn, spec, showValues)
	if err != nil {
		// A prediction failure is non-fatal: report unenforced with the reason.
		return unenforced(spec, "could not check for duplicates: "+err.Error())
	}
	if count == 0 {
		return store.DerivedObjectOutcome{Spec: spec, State: store.DerivedCreated, WouldChange: true}
	}
	out := unenforced(spec, "pre-existing duplicate values would block this constraint")
	out.BlockingCount = count
	out.SampleValues = samples
	out.WouldChange = true
	return out
}

// unenforcedFromCreateErr turns a failed CREATE into an unenforced outcome. It
// re-queries for the blocking duplicate groups so the operator learns WHY the
// index could not be built (a count, and sample values only on opt-in).
func unenforcedFromCreateErr(
	ctx context.Context, conn *pgxpool.Conn, spec store.DerivedObjectSpec, createErr error, showValues bool,
) store.DerivedObjectOutcome {
	out := unenforced(spec, "constraint could not be created: "+createErr.Error())
	if count, samples, err := uniqueViolators(ctx, conn, spec, showValues); err == nil && count > 0 {
		out.Reason = "pre-existing duplicate values block this constraint"
		out.BlockingCount = count
		out.SampleValues = samples
	}
	return out
}

// uniqueViolators counts the value groups that violate the (type, property)
// uniqueness (>1 row sharing a non-empty value), and — only when showValues —
// returns a bounded sample of those values. The values are entity content, so
// they are surfaced ONLY on explicit operator opt-in and never by default.
func uniqueViolators(
	ctx context.Context, conn *pgxpool.Conn, spec store.DerivedObjectSpec, showValues bool,
) (count int, samples []string, err error) {
	const q = `
		SELECT properties->>$2 AS val, count(*) AS n
		FROM entities
		WHERE type = $1 AND properties->>$2 <> '' AND properties->>$2 IS NOT NULL
		GROUP BY properties->>$2
		HAVING count(*) > 1
		ORDER BY n DESC
		LIMIT 100`
	rows, err := conn.Query(ctx, q, spec.Type, spec.Property)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()

	const maxSamples = 5
	for rows.Next() {
		var val string
		var n int
		if err := rows.Scan(&val, &n); err != nil {
			return 0, nil, err
		}
		count++
		if showValues && len(samples) < maxSamples {
			samples = append(samples, val)
		}
	}
	return count, samples, rows.Err()
}

// SetUniqueSpecProvider records the current metamodel's unique (type, property)
// pairs so the write path can attribute a derived-unique-index violation to a
// property (see mapUniqueViolation). The wiring layer calls it after
// construction and again on a metamodel reload. Passing nil clears it (a
// violation then degrades to a property-less UniquePropertyError). It is safe to
// call concurrently with writes: the value is published via an atomic pointer.
func (s *Store) SetUniqueSpecProvider(specs []store.DerivedObjectSpec) {
	if specs == nil {
		s.uniqueSpecs.Store(nil)
		return
	}
	cp := append([]store.DerivedObjectSpec(nil), specs...)
	s.uniqueSpecs.Store(&cp)
}

// currentUniqueSpecs returns the last-published unique pairs (nil if none).
func (s *Store) currentUniqueSpecs() []store.DerivedObjectSpec {
	if p := s.uniqueSpecs.Load(); p != nil {
		return *p
	}
	return nil
}

// mapConflict maps a write error to the right conflict sentinel. A non-23505
// error is returned unchanged (nil stays nil). A 23505 on an owned derived
// unique index (rela_derived_uniq__*) becomes a store.UniquePropertyError naming
// the property; any other 23505 (the entity-id indexes, the primary key) stays
// store.ErrConflict — preserving the existing "already exists" contract. The
// property is recovered by recomputing every current-metamodel unique index
// name and matching (NO persisted registry), so a rolling deploy against a
// peer-created index this process's metamodel does not know about degrades to a
// property-less UniquePropertyError (still a conflict), never a 500 or a
// misattributed property (RR-B5Y6DZ). The raw pgErr.Detail — which echoes the
// colliding VALUE — is never propagated (enumeration-oracle, RR-3NB0P9).
func (s *Store) mapConflict(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return err
	}
	return mapUniqueViolation(pgErr.ConstraintName, s.currentUniqueSpecs())
}

// mapUniqueViolation classifies a 23505's constraint name. See mapConflict.
func mapUniqueViolation(constraintName string, metaPairs []store.DerivedObjectSpec) error {
	if !strings.HasPrefix(constraintName, derivedUniquePrefix) {
		return store.ErrConflict
	}
	for _, p := range metaPairs {
		if uniqueIndexName(p.Type, p.Property) == constraintName {
			return store.UniquePropertyError{Property: p.Property}
		}
	}
	// Owned-prefix index we can't attribute to a current declaration: still a
	// uniqueness conflict, just unnamed.
	return store.UniquePropertyError{}
}

// unenforced builds a bare unenforced outcome.
func unenforced(spec store.DerivedObjectSpec, reason string) store.DerivedObjectOutcome {
	return store.DerivedObjectOutcome{Spec: spec, State: store.DerivedUnenforced, Reason: reason, WouldChange: true}
}

// sortedNames returns the map keys sorted, for deterministic output.
func sortedNames(m map[string]store.DerivedObjectSpec) []string {
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// quoteIdent double-quotes a SQL identifier, doubling any embedded quote. Used
// for the index NAME (which we generate, so it is already safe, but quoting is
// correct hygiene).
func quoteIdent(id string) string {
	return `"` + strings.ReplaceAll(id, `"`, `""`) + `"`
}

// quoteLiteral single-quotes a SQL string literal, doubling any embedded single
// quote. Used for the entity type and JSON property key interpolated into DDL
// (which cannot be bound as parameters). The metamodel's ValidateSchemaName has
// already rejected quotes/backslashes/control chars, so this is defense in depth.
func quoteLiteral(s string) string {
	return `'` + strings.ReplaceAll(s, `'`, `''`) + `'`
}
