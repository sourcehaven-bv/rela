---
id: TKT-IFHO2L
type: ticket
title: Relation-based validation gates are silently dropped; port workflow gates to Lua + enforce done-before-PR
kind: refactor
priority: high
effort: m
status: done
---

## Description

Two linked defects in the meta-tooling let PRs merge with tickets that are
neither marked `done` nor actually gated.

### Defect 1 — 14 relation-based validation rules are silently no-ops (root cause)

`internal/metamodel/types.go` `ValidationRule` has **no `relations` field**. It
unmarshals only `name`, `description`, `entity_type`, `when`, `then`, `content`,
`severity`, `lua`/`lua_file`/`lua_args`. Any rule that carries a `relations:`
block has that block **silently discarded at YAML parse time** (the loader is
not strict about unknown keys within a rule).

So a gate like:

```yaml
- name: done-ticket-needs-review-done
  entity_type: ticket
  when: ["status=done"]
  relations:                 # <-- DROPPED at parse; never evaluated
    has-review:
      where: ["status=done"]
      min: 1
  severity: error
```

parses as `when: [status=done]` with no `then` → vacuously true → **always
passes**.

**14 rules in `tickets/metamodel.yaml` are affected** (all inert today):
`planning-ticket-needs-checklist`, `in-progress-ticket-needs-planning-done`,
`review-ticket-needs-implementation-done`, `done-ticket-needs-review-done`,
`done-enhancement-needs-docs-done`, `done-docs-ticket-needs-docs-done`,
`analyzing-bug-needs-checklist`, `in-progress-bug-needs-analysis-done`,
`review-bug-needs-implementation-done`, `done-bug-needs-review-done`,
`done-ticket-no-open-critical-responses`,
`done-ticket-no-open-significant-responses`,
`done-bug-no-open-critical-responses`, `done-bug-no-open-significant-responses`.

The entire relation-based quality-gate layer described in the project CLAUDE.md
("Validation Gates: tickets cannot be marked done if they have open
critical/significant review responses", "done tickets must have completed review
checklist") is **decorative**. Evidence: **32 of 176 `done` tickets have no
`has-review` relation at all**, and `rela validate --check cardinality --check
properties --check validations` passes clean on every one of them.

### Defect 2 — tickets are not marked `done` before the PR

The intended model (confirmed with the maintainer) is: **finish the work → set
the ticket `done` and validate it → *then* push the PR** ("done = ready to
merge"). But nothing enforces the order:

- `/pr` (`.claude/commands/pr.md`) never touches ticket status. It pushes and
opens the PR.
- The `review → done` transition lives in a *separate* step (`/verify` Step 2
"For transition to `done`", and the tail of `/ticket`) that runs *after* the
work and is routinely skipped.
- The CI `rela-tickets` job only asserts "*some* bug/feature/ticket file changed"
— not that the referenced ticket is `done`. It is also **skipped entirely for
`chore/*` and `dependabot/*` branches**.

Result: 23 tickets stranded in `ready`, plus the whole `chore/*` class merges in
whatever state (this was how TKT-QXHFJZ merged as a stale `ready` ticket in
rela#1108 — the incident that surfaced this).

## Approach

### Part A — make the 14 gates fire (Lua port, now)

Rather than block on native engine support (see the follow-up ticket below),
port the 14 relation-based gates to `lua_file:` rules, which the engine **does**
run. Proven working: a probe `lua_file` rule using
`rela.get_relations{from=entity.id, type="has-review"}` +
`rela.get_entity(rel.to)` correctly flagged all 32 offending `done` tickets with
`severity: error` and exit 1.

- Add one or a few parameterized Lua validators under `tickets/validations/`
(e.g. `require-linked-status.lua` taking relation-type + required target status
+ min, and a `no-open-responses.lua` variant), reused across the 14 rules via
`lua_args`.
- Replace each rule's dropped `relations:` block with `lua_file:` + `lua_args:`.
- The 32 pre-existing offenders will now fail — triage: either backfill their
review checklists or accept a one-time data-cleanup pass (separate decision).

### Part B — enforce done-before-PR (command + CI)

- **`/pr` gate**: at the top of `.claude/commands/pr.md`, look up the ticket
(`show_entity`), require `status=done` **and** a clean scoped `rela validate
--check validations` for it; abort with remediation steps otherwise. Fast
feedback at the point agents act.
- **CI backstop**: extend the `rela-tickets` job so the ticket referenced by the
PR must be `status=done` and validate-clean before merge (not merely
"modified"). Decide deliberately whether the `chore/*` exemption stays — it is
the hole TKT-QXHFJZ slipped through. (Related: BUG-8D16G, BUG-29VYB touch the
same CI job and its exemptions.)

## Follow-up ticket to file — native relation-cardinality validation

> **Title:** Add native relation-cardinality support to validation rules
> (`relations:` block on `ValidationRule`)
>
> **Problem:** Validation rules cannot express "entity must have min/max linked
> entities of relation type R, optionally filtered by the target's own
> properties". The `relations:` block already used throughout
> `tickets/metamodel.yaml` is silently dropped because `ValidationRule`
> (`internal/metamodel/types.go`) has no such field. The gates were ported to Lua
> as a stopgap; this ticket replaces the Lua stopgap with first-class engine
> support so rules stay declarative.
>
> **Scope:** (1) Add a `Relations map[string]RelationConstraint` field to
> `ValidationRule` with `min`/`max`/`where` (target-property filters reusing the
> existing `--where` predicate parser). (2) Evaluate it in `internal/analysis`
> alongside `when`/`then`, emitting an error/warning per the rule severity.
> (3) Make the metamodel loader **reject unknown keys** inside a validation rule
> so a future typo fails loudly instead of silently (this bug's real root cause).
> (4) Migrate the 14 Lua-ported gates back to the declarative `relations:` form
> and delete the stopgap Lua. (5) Conformance tests: a `done` ticket without a
> completed `has-review` must produce an error; strict-loader test for the
> unknown-key rejection.
>
> **Why not now:** larger Go change across metamodel + analysis + loader-strictness
> with its own test surface; the Lua port restores the gates immediately with no
> engine risk.

## Acceptance Criteria

- [ ] All 14 relation-based gates actually evaluate (Lua port); a `done` ticket
lacking a completed review checklist fails `rela validate` with exit 1.
- [ ] `rela validate` in CI flags the pre-existing 32 offenders (or they are
backfilled/triaged as a deliberate, recorded decision).
- [ ] `/pr` refuses to proceed unless the referenced ticket is `status=done` and
validate-clean, with actionable remediation output.
- [ ] CI `rela-tickets` job fails a PR whose referenced ticket is not `done` +
validate-clean; the `chore/*` exemption is either closed or explicitly justified
in the job.
- [ ] Follow-up ticket for native `relations:` support is filed and linked.
- [ ] No regression: `just ci` green; existing property/`then`-based rules
unaffected.
