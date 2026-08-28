---
id: TKT-DOCASSERT
type: ticket
title: Executable manuals — assertions in the rela-docs doc language
kind: enhancement
priority: high
effort: l
status: review
---

Make a manual prove its own claims. `internal/docs` already runs Lua islands
against a seeded in-memory graph and can drive the real SPA; it cannot yet
ASSERT anything. Adding assertion verbs turns every manual into a test whose
failure is a readable prose diff.

## Why

Documentation and tests drift apart because they are separate artifacts. Every
world bug found during the FEAT-9CD2MX demo review was something the
documentation would have *claimed* — "a published world shows only published
content" — while the code did otherwise. A manual that executes its claims
closes that gap by construction: there is one artifact, so there is nothing to
drift.

Full analysis, options and rejected alternatives: `.ignored/TESTING.md`.

## What exists already

- **Doc language with Lua islands** over a seeded graph
  (`internal/docs/preprocess.go:1-12`). Statement islands run for side effects;
  echo islands substitute a value mid-text.
- **`create{}` / `link{}`** seed entities and relations, accumulating into
  `seedOps` (`module.go:105,121`).
- **`screenshot{}`** opens the real SPA in a real browser and captures
  (`screenshot.go:93`), taking `view`, `type`, `entity`, `form`, `as`, `clip`.
  **`as=` is a principal**, so ACL is already expressible.
- **`chromedp.Evaluate`** already runs JS in the live page
  (`docscapture/annotate.go:26`) — element introspection needs verbs, not
  infrastructure.
- **A golden-file gate**: `just docs-check` regenerates and diffs
  (`justfile:299-303`). Echo islands are therefore ALREADY repr-assertions.

## Scope

**1. `open{}` + a current page.** `docRuntime` is stateful across islands
(`runtime.go:61-86`), so an open page can live there like `seedOps` does.
Subsequent assertions target it, so a paragraph names its screen once.

**2. `shows{}` — every argument optional.** Assert only what the paragraph is
about. Claims: `text`/`no_text`, `button`/`no_button`, `link`/`no_link`,
`enabled`/`disabled`, `exactly` (for lists), `selector` (escape hatch),
`width` (viewport).

Key on **accessible name / visible label** by default, not CSS — it survives
restyling and reads naturally in a manual. Note `data-testid` appears in only 4
component files, so this likely needs an accessibility pass first; that is a
product improvement in its own right, not test scaffolding.

`no_button` is the important one: this codebase's rule is that a denied action
is ABSENT, not disabled (RULING 11), so the negative claim encodes the contract.

**3. `api{}` — contract assertions.** `status`, `error` (the machine-readable
code, not the prose), `world`, `as`, `body`. Needs no browser, so unlike
`shows{}` it can gate CI unconditionally. Also `identical_to={...}` for the
existence-oracle property — two responses byte-identical — which is currently
pinned only in Go (`viewworld_absent_test.go:120`).

**4. `refuses{}` — load-time refusals.** `otherwise:` is mandatory
(`metamodel/loader.go:767`); a copy targeting a guarded face must carry a guard
(`metamodel/copies.go:153`, RULING 18). No server, no store — the cheapest
assertions in the system, and RULING 18 was discovered by an agent tripping
over it rather than by reading docs that mentioned it.

**5. `world=` on the capture spec.** Absent today (`grep world screenshot.go`
→ nothing), so a manual cannot open a page in a world at all.

**6. Run one manual against several backends.** History is postgres-only; the
per-face index differs bleve vs pg. Gate backend-specific claims the way
`storetest.Capabilities` already does for the store suite. **CI runs all main
backends.**

## Rules this must obey

- **A call that asserts nothing is an ERROR.** `shows{}` with no claim, or an
  omitted target with no page open, must fail rather than pass silently. This
  is the failure mode the FEAT-9CD2MX work hit repeatedly — a check that
  passes while checking nothing.
- **A refusal assertion needs a positive control.** `status=422` passes just as
  happily against a server that 422s everything.
- **Prefer `exactly` for lists.** `contains={"CTL-2"}` would have PASSED on the
  relation-leak bug, which returned `["CTL-2","CTL-1","CTL-2"]`. The bug was
  over-inclusion, which only an exact claim sees.
- **Failure output prints the page and the seed**, not just the claim. Most
  confusion during the demo review came from not knowing which faces existed;
  `seedOps` is already recorded and available.

## Deliberately NOT in scope

- **Pixel-diffing screenshots.** Fails on fonts and styling, training people to
  accept diffs — which destroys the golden-file gate where a diff means
  something. Screenshots for humans, structural assertions for the machine.
- **An auto-accept `--update-docs`.** Doctest's paste-the-output culture would
  industrialise the exact failure this project keeps hitting: an auto-accepted
  repr of the relation-leak bug would have pinned the duplicate as correct, and
  the test would then defend it.
- **Ellipsis elision.** If repr-capture ever lands, use a NAMED exclusion
  (`except={"updated_at"}`) — the precedent is `stripInstance`
  (`internal/dataentry/acl_get_test.go:77`), which excludes one field with a
  stated reason. (The originally-cited `bodyWithoutInstance` in
  `viewworld_absent_test.go` was deleted in `d8154c57`.) An ellipsis silently swallows new fields; several FEAT-9CD2MX bugs
  were EXTRA content an ellipsis would have hidden.

## First step

`shows{}` with `text`/`no_text` asserting on the API response, plus `world=`.
No browser, runs everywhere, and makes a worlds manual executable immediately.
Then write ONE manual — the worlds guide — as the proof: it is the subsystem
with the worst docs-to-behaviour drift, and `.ignored/WORLDS-DEMO-ISSUES.md`
supplies the cases.

## Delivered so far

Store-free assertions, on develop (no dependency on the worlds epic):

- **`shows{type, contains|absent|exactly}`** — asserts which entities exist.
  `exactly = {}` is a real claim ("this type is empty"), distinct from no claim.
- **`refuses{}` / `permits{}`** — authorization claims evaluated through
  `acl.Declarative`, the same path the write path uses, so a manual fails if a
  grant widens OR if the gate stops being consulted. `because=` also pins WHY,
  since a deny from an unintended rule is a green check over a regression.
- **`api{path, status, error, as, identical_to}`** — real requests over a
  seeded temp project. No browser and no built frontend, so it gates CI.
  `identical_to` expresses the existence-oracle property, excluding the
  problem-details `instance` (it echoes the request url, so a raw compare could
  never pass) — the named-exclusion discipline this ticket asks for above.
- **The "asserts nothing is an ERROR" rule** on every verb, mutation-tested.
- **Proof**: the example handbook (`prototypes/data-entry/manual/`) now checks
  its own access-control prose. Widening `viewer` in `acl.yaml` makes it refuse
  to build, naming the rule that fired.

Incidental fix: `docscapture`'s test skip guard stat'd `metamodel.yaml`,
renamed to `schema.yaml` in TKT-FNARO6 — those tests had been skipping
silently. Fixing it raised package coverage 27% → 44%.

## Not yet built, and why

- **`open{}` + browser claims (`text`/`button`/`link`, item 2).** Needs the
  accessibility pass this ticket already flags as a prerequisite: assertions
  should key on accessible names, and `data-testid` appears in only 4
  components. Doing it before that pass would bake CSS selectors into manuals.
- **`world=` (item 5).** Blocked: content states are not on develop —
  `store.EntityQuery` has no world field and `entity.Pointer` does not exist
  there. Buildable as soon as the worlds epic merges; the worlds proof-manual
  in "First step" depends on this.
- **Multiple backends (item 6).** Not reachable on the current architecture:
  the docs pipeline has no backend concept, and both `screenshot{}` and `api{}`
  are deliberately unavailable on the `postgres` build because seeding a
  "throwaway" project would write into the live database. Running one manual
  across backends needs an ephemeral-postgres seam first — a ticket of its own,
  not a line item here.

Related: [[FEAT-G4VO53]], [[test-fixtures]], [[ci-pipeline]].
