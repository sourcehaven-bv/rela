---
id: RR-5LZWX8
type: review-response
title: ConstraintName population for expression-index 23505 is unverified — discriminator may collapse
finding: Error-mapping branches on pgErr.ConstraintName, but no repo code reads it and there's no evidence pgx v5 populates it for EXPRESSION-index violations. entities_id_lower_key is itself an expression index already mapped to ErrConflict, so the discriminator is expression-vs-expression and collapses if ConstraintName is empty. Verify empirically (spike) before committing.
severity: critical
resolution: 'SPIKE RUN (pgx v5.10.0, Postgres 15.17). A partial expression unique index of the reconciler''s EXACT shape (`CREATE UNIQUE INDEX rela_derived_uniq__abc123 ON entities (type,(properties->>''email'')) WHERE type=''persoon'' AND properties->>''email''<>'''' AND IS NOT NULL`) was violated; pgconn.PgError came back Code=23505, ConstraintName="rela_derived_uniq__abc123" — POPULATED and exact. Also verified: lower(id) expression index → ConstraintName="entities_id_lower_key"; PK → "entities_pkey"; plain partial column index → its name. So the discriminator (branch PK/entities_id_lower_key → ErrConflict vs rela_derived_uniq__* → ErrUniqueProperty by ConstraintName) is SOUND. The design is unblocked on this axis. NOTE: PgError.Detail echoes the colliding value (`Key (type,(properties->>''email''))=(persoon, a@x.com) already exists.`) — reinforces RR-3NB0P9: never surface raw Detail to a client (enumeration oracle); map to the property-only ValidationErrorUnique.'
status: addressed
---

The whole error-mapping design branches on `pgErr.ConstraintName` to route
`entities_id_lower_key`/PK → `ErrConflict` vs `rela_derived_uniq__*` →
`ErrUniqueProperty`. No code in the repo reads `ConstraintName` today
(`isUniqueViolation` at pgstore/migrate.go:162 only checks Code=="23505"), so
there is NO in-repo evidence pgx v5 populates it for an EXPRESSION unique index
violation. Crucially `entities_id_lower_key` is ITSELF an expression index (`ON
entities (lower(id))`, migration 0007) already mapped to ErrConflict — so the
discriminator is "expression index vs expression index," entirely dependent on
ConstraintName being non-empty and correct. If pgx returns empty ConstraintName
for expression-index violations, the two become indistinguishable and every
derived-unique violation mis-maps to a 409.

REQUIRED: verify empirically against pgx v5 BEFORE committing to this design — a
spike/AC, not an assumption. Postgres emits constraint_name in the error fields
for index violations; "pgx surfaces it in PgError.ConstraintName" is the
unproven load-bearing claim.
