---
id: TKT-0RBFN0
type: ticket
title: 'Apply relation-level visible: redaction to the gated read paths'
kind: enhancement
priority: medium
effort: s
status: backlog
---

Relation-level `visible:` grants are honored on the dataentry wire but not on
the appbuild-wired identity-bearing read paths (MCP, scheduler, automation
cascade). Filed from IB-review finding #1 on PR #1400.

Follow-up to [[TKT-BUYEW1]], which closed the same gap for **entity**
properties. This is the relation-side remainder, and was named as an explicit
carve-out in that PR's `Services.GatedReads` godoc rather than silently left.

## Problem

`acl.RelationGrant.Visible` compiles into the policy and
`affordances.PolicyResolver.RelationFieldVerdicts` resolves it — but only
`internal/dataentry` calls it (`affordances.go:987`). On the gated read paths
relation meta is served unredacted.

The gap is **wider than the review states**. It is not only `GetRelation`:

| Site | Path | Row-gated? | Field-redacted? |
|---|---|---|---|
| `gatedGraphReader.GetRelation` | `g.raw` (store) | no | no |
| `gatedGraphReader.ListRelations` | `g.rows` | yes | **no** |

`ListRelations` goes through the gated reader, so a hidden edge is not listed —
but `visibility.PolicyReader` implements only `FilterRelations` (row-level, both
endpoints). There is no relation *field* redaction anywhere in
`internal/visibility`, so a surviving edge carries all of its meta.

Both sites feed the same three consumers, and all three send data onward (a
prompt, a webhook, an automation write), so an unredacted relation property can
leave the system entirely — the same argument that made [[TKT-BUYEW1]] worth
doing.

## Note on the existing godoc rationale

`gatedGraphReader`'s comment defends the raw `GetRelation` on the grounds that
a relation is addressed by its two endpoint ids, so reading one confirms
nothing the caller could not already name. That argument is sound, but it is
about **row**-level exposure (does this edge exist). It does not defend serving
the edge's **meta values** unredacted. Fixing this ticket means correcting that
comment's scope, not overturning its conclusion — `GetRelation` may well stay
raw at the row level.

## Design constraint (found while triaging)

`RelationFieldVerdicts(ctx, from *entity.Entity, relType, metaKeys)` needs the
**live FROM entity**, not just its id: role grants are keyed by the FROM
entity's type and the `when:` predicates bind against it. The entity path had
its subject in hand; here a redactor must resolve `from` first.

That is the real cost, and it is what makes this ticket `s`-or-larger rather
than a one-line rewire:

- a per-relation FROM lookup on a list path is an N+1 read — needs batching or
  a per-call cache, since `ListRelations` is an `iter.Seq2`
- resolution must use a **raw** read, not the gated reader: the FROM entity may
  itself be row-hidden while the relation is legitimately visible via the TO
  side, and a nil `from` makes `RelationFieldVerdicts` return nil = redact
  nothing, which is **fail-open**

Fail-open on a nil FROM is the trap to design against explicitly.

## Acceptance criteria

- [ ] Relation meta is redacted per `visible:` on `GetRelation` and
      `ListRelations` for all three gated consumers
- [ ] A nil / unresolvable FROM entity fails **closed**, not open
- [ ] Mutation-verified test: a relation-level `visible:` grant that hides a
      meta field must make the test fail when the redaction is reverted
- [ ] List-path test covers the `iter.Seq2` case, not only the single GET
- [ ] The `gatedGraphReader` godoc scope note and `GatedReads`' SCOPE paragraph
      are updated (both currently state this is unenforced)
- [ ] `docs/acl-security.md` "Relations have no field-level redaction" is
      corrected

## Out of scope

Relation **history** meta redaction. `RelationFieldVerdicts`' godoc records
that a deleted-source relation history serves no meta at all, so it never
reaches that resolver — a separate path with its own fail-closed reasoning.
