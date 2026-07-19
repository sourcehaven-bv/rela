---
id: TKT-0YBFT8
type: ticket
title: 'Metamodel doc-fields: top-level description, per-enum-value descriptions, transition help (rela-docs phase 1a)'
kind: enhancement
priority: medium
effort: s
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Problem

The `rela docs` generator (FEAT-G4VO53, phase 2) turns a deployment's schema
into end-user + operator docs. Per the research (RES-EK7LSA / Diátaxis), a
schema generates excellent *reference* but the *explanation* layer — what a
status means, why/when to make a transition, what the whole app is for — can't
be derived from structure. The metamodel has `Description` on
entities/properties/ relations/types and per-value `Labels`, but is missing
three prose fields the generator needs. This ticket adds them first (phase 1a)
so phase 2 renders a complete surface on day one.

## Scope

**In:**
1. `Metamodel.Description string` (top-level, `yaml:"description,omitempty"`) — a
paragraph describing the deployment/app.
2. Per-enum-value descriptions on `CustomType` — the *meaning* of each value
(distinct from `Labels`, which is display text). Shape: a `map[string]string`
keyed by value (mirrors `Labels`), e.g. `ValueDescriptions` / `descriptions:`.
3. `TransitionDef.Help string` (`yaml:"help,omitempty"`) — why/when to make this
move, beyond the verb `Label`.
4. Populate all three in the in-tree example project(s) so the generator has real
content and the fields have live usage examples.

**Out:**
- The `rela docs` generator itself (phase 2, separate ticket).
- ACL `RoleDef.Description` (phase 1b, separate ticket).
- Any `docs:` structured sub-block (research rejected A2 — keep it minimal).
- Enforcement/validation behavior changes — these are display-only; `analyze_*`
and write paths ignore them.

## Design notes (firm up in planning)

- All three are additive + optional; a metamodel without them loads and behaves
exactly as before. Same pattern as the existing `Description`/`Labels` fields.
- Confirm YAML round-trips and that the fields survive `metamodel` parse/marshal
and any migration/normalization passes.
- Naming: `ValueDescriptions` vs `Descriptions` for the CustomType map — pick to
read well in YAML (`descriptions:` keyed by value) and not collide with the
existing `Description` scalar on CustomType.
- Which example project(s) to populate: the in-tree ones the generator will be
demoed against (e.g. `tickets/` and/or a prototype project). Keep additions
realistic, not lorem ipsum.

## Acceptance criteria (firm up in planning)

1. `Metamodel.Description` parses from `description:` at the metamodel root and is
readable via the metamodel API; absent → empty, no behavior change.
2. CustomType carries per-value descriptions parsed from YAML, keyed by value,
readable via the metamodel API; absent → empty.
3. `TransitionDef.Help` parses from `help:` and is readable; absent → empty.
4. All three are backward-compatible: existing metamodels (no new fields) load
and validate unchanged; round-trip (parse→marshal) preserves them.
5. At least one in-tree example project populates all three fields with realistic
content.
6. Tests cover parse + absence + round-trip for each field.

## References

- Research: RES-EK7LSA (Diátaxis framing, the four gaps).
- Feature: FEAT-G4VO53 (rela-docs arc).
- Mirror the existing field shape in `internal/metamodel/types.go`
(`EntityDef.Description`, `CustomType.Labels`, `TransitionDef.Label`).
