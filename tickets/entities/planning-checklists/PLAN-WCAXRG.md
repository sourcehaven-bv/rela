---
id: PLAN-WCAXRG
type: planning-checklist
title: 'Planning: Typed state references and the store contract (Step 1)'
status: done
---

<!-- @managed: claude-workflow v1 -->

**Plan of record: `.ignored/dofyr1-plan.md`** — the full survey (citation
verification against post-merge develop), the 3-PR decomposition, and all nine
architect decisions (2026-08-19, incl. the Jeroen-confirmed Face-A
representation, DEC-0VGTF3) live there. This checklist summarizes and gates; the
plan file carries the detail.

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN scope — three stacked PRs, each keeping CI green:

- **PR-A (contract core + fs/mem + storetest):** `entity.Face` named
opaque type (canonical serialized coordinate, zero = default state; codec-only
construction; store equality-matches, never inspects — both rules load-bearing
godoc); boundary codec in `internal/entity` (grammar-only: base-id defers to
`ValidateID`, face lexical `[a-z][a-z0-9-]*`, coordinate-ready signature,
never emits empty); `Entity.Face` + `Relation.FromFace`; relation
`from_face:` frontmatter + `PAGE-1@draft.md` filename serialization; store
surface: `GetEntity` unchanged (= default state), new `GetEntityState(ctx, id
string, p entity.Face)`, `PutEntity` keys off `Entity.Face`, NO per-state
delete, `DeleteEntity`/rename cascade over ALL states + their relations;
`RelationQuery.FromFace *entity.Face` (nil = unfiltered — the
relation-side compat story, pinned); `EntityQuery.AllStates bool` (raw
storage-truth enumeration, NOT world resolution — godoc line mandatory);
`store.Event.Face`; fs/mem implementation incl. the
`relationMeta.FromFace` index fix; storetest `RunStateTests` behind
transitional `Capabilities.States` (`// TODO(TKT-DOFYR1-PR-B): remove`); pgstore
compiles + fails closed on faced writes.
- **PR-B (pgstore):** migration 0011 — compound PK `(id, face text
NOT NULL DEFAULT '')` on entities (Jeroen-confirmed), widened relations PK with
`from_face text NOT NULL DEFAULT ''`; `relationWhere` face match;
cascades; Event.Face + change-feed payload via codec; versioning checklist
(distinct rel_record_ids per FromFace, face on sweep scan +
DeletedRelations, copy-vs-sweep dedup); flip AND DELETE the Capabilities.States
flag; drop PR-A's rejection guard.
- **PR-C (relation scope + detection):** `scope: identity | content`
per relation type, default identity, declarative-only in Step 1;
undeclared-face detection — analyze finding (count + example ids per
undeclared face) + appbuild load WARNING (never refusal), store stays
metamodel-ignorant, enumeration via AllStates.

OUT of scope (all recorded decisions):

- `EntityQuery` world-scope field — CUT to TKT-WAV8XP's first PR
(decision 7; designed with the resolver's compile target).
- Per-state delete (first consumer is Step 4).
- Any worlds/resolver/metamodel-faces work (Step 2), copy kernel
(Step 3+), ACL world grants (TKT-DN37J2), attachment link sets (§6.3).
- Re-key tooling for faces — FEAT-T3EF5A's migration system;
DOFYR1 ships detection only (DEC-0VGTF3).

**Acceptance Criteria:**

1. Faceless projects behave byte-identically: all 84 non-test
`EntityQuery` sites and every relation query unchanged (zero values = today's
semantics) — pinned by the existing suites staying green plus explicit
nil-semantics storetest cases.
2. A state is addressable by `(id, face)` through the store surface;
bare-id `GetEntity` returns the default state — storetest.
3. Delete and rename cascade over all states and their relations in
fs/mem (PR-A) and pg (PR-B) — storetest cascade cases, written before pg
implements.
4. Relation matching honors `FromFace` in BOTH implementations
(storeutil predicate + fs index + pg SQL) — storetest, incl. the indexed-query
case.
5. `AllStates: true` returns default + non-default rows; default query
returns only default — storetest.
6. Events fire per state with `Event.Face`; pg change-feed payload
round-trips through the codec (PR-B).
7. Codec: parse/format round-trip, grammar rejection (bad base-id, bad
face, double `@`), never emits empty face; an unusual-but-canonical
face value round-trips the store unchanged.
8. Undeclared stored face → analyze finding + load warning, never a
refusal (PR-C).

## Research

- [x] For larger features: research exists — RES-GFWP85 + RES-NH3P12
(feature-level, superseded in part by the v2 design doc) and the read-only
survey in `.ignored/dofyr1-plan.md` (citation verification against post-merge
develop, 2026-08-19)
- [x] ~~Searched for existing libraries~~ (N/A: domain type + store contract, internal)
- [x] Checked codebase for similar patterns or reusable code (survey: rename cascade sites, MatchRelation/relationWhere dual implementation, storetest harness + Capabilities, state.KV, feed separators)
- [x] ~~Looked for reference implementations in other projects~~ (N/A: contract dictated by design doc §2/§3/§6)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** `.ignored/dofyr1-plan.md` (survey), RES-GFWP85 / RES-NH3P12
(feature-level, via FEAT-9CD2MX)

**Existing Solutions:** see plan §1 (verification table) — every citation
verified against post-merge develop with drift documented; new findings: real
`ValidateID` pattern is `^[A-Za-z0-9][A-Za-z0-9_-]*$` (codec defers, never
restates), and fsstore's `relationMeta` index must gain `FromFace` (one
predicate, two data sources).

## Approach

- [x] Technical approach chosen and documented (plan §2, 3-PR split)
- [x] Approach builds on existing patterns (RenameEntity cascade model, storetest conformance discipline, DEC-ZBI39P layering untouched)
- [x] Alternatives considered (plan §3: all nine decision records with rejected alternatives — raw string vs opaque type, nullable vs sentinel, per-id enumeration vs AllStates, A+B-as-one-PR vs transitional capability)
- [x] Dependencies identified (internal/entity, internal/markdown, store + fsstore/memstore/pgstore + storeutil + storetest, appbuild/analyze for PR-C)

**Technical Approach:** see plan §2. Green-CI mechanism for the
storetest-before-second-backend constraint: transitional `Capabilities.States`
flag, one-commit-window discipline (PR-B flips and deletes it), pg fails closed
on faced writes in the window.

**Files to modify:** per PR — see plan §2 work lists.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- The serialized `@` form enters ONLY through the codec at declared
boundaries; `entity.ValidateID` keeps rejecting `@` on anything user-supplied;
`isSafePathSegment` keeps rejecting `@` globally (routes that accept the form
parse through the codec first — no route work in this ticket). Face grammar
is an allowlist (`[a-z][a-z0-9-]*`).
- fsstore filename parsing of `PAGE-1@draft.md` goes through the codec;
the joined form never leaks upward (§3.4).

**Security-Sensitive Operations:**

- Cascade delete must remain fail-secure (existing fsstore discipline:
entity not removed while a relation file could not be — extends to state files).
- No ACL changes in this ticket; visibility wrappers see states as
entities (row-gating unchanged); world-shaped grants are TKT-DN37J2.
- pg fail-closed guard in the PR-A→PR-B window prevents silent data
loss (rejected, never dropped).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion (above; storetest `RunStateTests` + codec unit tests + fs filename round-trip)
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (storetest IS the cross-backend integration contract; conformance suites run per backend incl. DB-gated pg)

**Test Scenarios:** AC-numbered above; the conformance cases are the
load-bearing artifact (they land in PR-A, before pg implements).

**Edge Cases:**

- Default-state addressing via bare id when non-default states exist.
- Rename with states: all state files/rows re-keyed; relations of every
state re-keyed; `rel_record_id` continuity (PR-B).
- Delete with states: every state + its relations gone; fail-secure
ordering.
- Relation on a non-default tail found via fs INDEXED query (the
relationMeta fix).
- Unusual-but-canonical face value round-trips (store opacity).
- `PAGE-1@draft.md` beside `PAGE-1.md`; file watcher sees state files.
- Event per state with correct Face.

**Negative Tests:**

- Codec rejects: bad base-id (defers to ValidateID), bad face chars,
uppercase face, leading digit/dash face, `a@b@c`, empty face
serialization.
- pg (window): faced write rejected with clear error.
- Undeclared face: warning + finding, never refusal (PR-C).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Contract drift between the two relation-match implementations →
storetest cases land first (the ticket's own discipline).
- Compat regression on the 84 query sites → zero-value semantics +
existing suites + explicit nil/default cases.
- Transitional capability outliving its window → TODO marker + PR-B
description notes removal (architect-mandated discipline).
- Stacked-PR churn (ancestors merging under us) → rebase promptly on
ancestor merge; diffs collapse (the #1381 pattern).
- develop moving under the stack (as TKT-80EWGM did) → full gates re-run
after every rebase/merge-from-develop.

Effort: xl (matches ticket property; the 3-PR split bounds each unit).

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- Godoc is the primary artifact (entity.Face, codec, store contract,
AllStates — load-bearing rules per decisions 4/6/8 + the amendment).
- [x] docs/metamodel.md — done in PR-C (relation `scope:` is the
first user-visible metamodel surface; faces-per-type declaration is Step 2,
not this ticket)
- [x] ~~docs/cli-reference.md~~ (N/A: no new commands in Step 1)
- [x] ~~docs/data-entry.md~~ (N/A: no UI surface)
- [x] ~~CLAUDE.md~~ (defer: a store-contract note may be warranted at stack end; decide in PR-C's docs checklist)
- [x] ~~README.md~~ (N/A)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan (the two open ones answered same-day by design-doc amendments)

**Design Review Findings:**

- RR-AUF5ZC (significant, addressed — doc §6 amended 2026-08-20):
store ENFORCES the row-family invariants (Put rejects headless states and type
mismatches); fsstore load tolerates + analyze surfaces (PR-C); cascades
defensive.
- RR-IVIPQA (significant, addressed — doc §3.2 amended 2026-08-20):
face grammar tightened to `[a-z][a-z0-9]*(-[a-z0-9]+)*` exactly as proposed;
implemented in face.go.
- RR-R6G2VM (significant, addressed in plan): fs relation keys/filenames
must widen with the tail face (fsstore/relation.go:17 collision) — PR-A work
item + storetest case.
- RR-8U1PE2 (significant, addressed in plan): search indexers key by
bare id (linearsearch.go:27) — per-state events would overwrite the default
face; Step-1 indexing observers skip non-zero-Face events, pinned by test.
Per-world indexing stays Step 5.
- Minor (in plan, no entities): entity state files — filename
authoritative, no `face:` entity frontmatter key; verify loader behavior on
unparseable-id files for the downgrade story; rename collision checks the
target's whole state family.
