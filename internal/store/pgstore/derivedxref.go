package pgstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// External-ref unique indexes (TKT-SM20FG): the database backstop for the
// entitymanager's (system, id) check, per (type, property). Under their own
// prefix, so neither the `unique:` rule nor the query-index rules can drop
// them, and so a violation is attributed by prefix.

// derivedXrefPrefix is the name namespace the external-ref rule owns.
const derivedXrefPrefix = "rela_derived_xref__"

// xrefIndexShape versions the index DEFINITION; bump it when the DDL
// changes, so the reconciler renames, creates and drops (see
// uniqueIndexShape).
const xrefIndexShape = "xref-id-per-face-v1"

// xrefIndexName is the deterministic index name of one external-ref spec.
func xrefIndexName(entityType, property string) string {
	sum := sha256.Sum256([]byte(entityType + "\x00" + property + "\x00" + xrefIndexShape))
	return derivedXrefPrefix + hex.EncodeToString(sum[:16])
}

// createXrefIndexDDL is the partial unique index over the `id` entry. The
// key expression and the jsonb_typeof guard are spelled as keyEqualCond
// spells them, so a [store.PropKeyEqual] lookup is an index scan. Only a
// string id is indexed: the write path refuses any other shape, and a
// legacy non-string value must not block the index.
func createXrefIndexDDL(name string, spec store.DerivedObjectSpec) string {
	prop := quoteLiteral(spec.Property)
	return fmt.Sprintf(
		`CREATE UNIQUE INDEX IF NOT EXISTS %s ON entities (type, (properties -> %s ->> 'id'), face) `+
			`WHERE type = %s AND jsonb_typeof(properties -> %s -> 'id') = 'string'`,
		quoteIdent(name), prop, quoteLiteral(spec.Type), prop)
}

// reconcileXrefIndexes converges the [store.DerivedExternalRefUnique]
// specs, like reconcileQueryIndexes, with a blocker count when existing
// duplicates prevent a create.
func reconcileXrefIndexes(
	ctx context.Context, conn *pgxpool.Conn, desired []store.DerivedObjectSpec, opts store.ReconcileOptions,
) ([]store.DerivedObjectOutcome, error) {
	desiredByName := make(map[string]store.DerivedObjectSpec)
	var outcomes []store.DerivedObjectOutcome
	for _, spec := range desired {
		if spec.Kind != store.DerivedExternalRefUnique {
			continue
		}
		if !safeDDLName(spec.Type) || !safeDDLName(spec.Property) {
			outcomes = append(outcomes, unenforced(spec, "unsafe name for DDL"))
			continue
		}
		desiredByName[xrefIndexName(spec.Type, spec.Property)] = spec
	}

	actual, err := listOwnedIndexes(ctx, conn, derivedXrefPrefix)
	if err != nil {
		return nil, fmt.Errorf("pgstore: reconcile: list external-ref indexes: %w", err)
	}
	for name := range actual {
		if _, ok := desiredByName[name]; ok {
			continue
		}
		out := store.DerivedObjectOutcome{
			Spec: store.DerivedObjectSpec{Kind: store.DerivedExternalRefUnique}, State: store.DerivedDropped,
			Reason: "index " + name + " no longer declared", WouldChange: opts.DryRun,
		}
		if !opts.DryRun {
			if _, dropErr := conn.Exec(ctx, `DROP INDEX IF EXISTS `+quoteIdent(name)); dropErr != nil {
				return nil, fmt.Errorf("pgstore: reconcile: drop %s: %w", name, dropErr)
			}
		}
		outcomes = append(outcomes, out)
	}
	for _, name := range sortedNames(desiredByName) {
		spec := desiredByName[name]
		if _, ok := actual[name]; ok {
			outcomes = append(outcomes, store.DerivedObjectOutcome{Spec: spec, State: store.DerivedEnforced})
			continue
		}
		if opts.DryRun {
			outcomes = append(outcomes, predictXref(ctx, conn, spec))
			continue
		}
		if _, createErr := conn.Exec(ctx, createXrefIndexDDL(name, spec)); createErr != nil {
			out := unenforced(spec, "index could not be created: "+createErr.Error())
			if count, cErr := xrefViolators(ctx, conn, spec); cErr == nil && count > 0 {
				out.Reason = "pre-existing duplicate external ids block this constraint"
				out.BlockingCount = count
			}
			outcomes = append(outcomes, out)
			continue
		}
		outcomes = append(outcomes, store.DerivedObjectOutcome{Spec: spec, State: store.DerivedCreated})
	}
	return outcomes, nil
}

func predictXref(ctx context.Context, conn *pgxpool.Conn, spec store.DerivedObjectSpec) store.DerivedObjectOutcome {
	count, err := xrefViolators(ctx, conn, spec)
	if err != nil {
		return unenforced(spec, "could not check for duplicates: "+err.Error())
	}
	if count == 0 {
		return store.DerivedObjectOutcome{Spec: spec, State: store.DerivedCreated, WouldChange: true}
	}
	out := unenforced(spec, "pre-existing duplicate external ids would block this constraint")
	out.BlockingCount = count
	return out
}

// xrefViolators counts the (id, face) groups holding more than one row: the
// rows the index would reject. Its WHERE must stay the index predicate.
// Ids are entity content, so no sample values are reported.
func xrefViolators(ctx context.Context, conn *pgxpool.Conn, spec store.DerivedObjectSpec) (int, error) {
	const q = `
		SELECT count(*) FROM (
			SELECT 1 FROM entities
			WHERE type = $1 AND jsonb_typeof(properties -> $2 -> 'id') = 'string'
			GROUP BY properties -> $2 ->> 'id', face
			HAVING count(*) > 1
		) g`
	var count int
	err := conn.QueryRow(ctx, q, spec.Type, spec.Property).Scan(&count)
	return count, err
}
