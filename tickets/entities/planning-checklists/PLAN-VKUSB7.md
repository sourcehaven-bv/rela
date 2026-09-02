---
id: PLAN-VKUSB7
type: planning-checklist
title: 'Planning: Plumb AutomationName through automation.Result for per-action audit attribution'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** Audit records for cascade-created relations and entities carry the
generic `triggered_by: "automation"`, so an operator filtering the log cannot
tell *which* automation caused a given write in a project with several `on:
created` rules. Scripted actions already carry `automation:<name>`. GitHub issue
#764.

**Root cause, precisely.** It is not the audit layer.
`cascadeHost.recordCascade` (`internal/entitymanager/cascadehost.go:258-274`)
already does the right thing: it uses a ctx-carried `triggered_by` when present
and only falls back to `"automation"` when there is none. And
`executeScriptActions` (`internal/autocascade/runner.go:270`) already wraps ctx
per action with `audit.WithTriggeredBy(ctx,
"automation:"+action.AutomationName)`. The mechanism exists and works — the
*name* just never reaches the non-scripted paths, because
`automation.Result.RelationsToCreate` is a bare `[]*entity.Relation` and
`automation.EntityToCreate` has no name field. `LuaToExecute` has one; its two
siblings do not.

**Scope — IN:**

- `automation.EntityToCreate` gains `AutomationName`; relation creations gain an
equivalent carrier.
- Both set at the engine seam (`executeAction`), where `automationName` is
**already a parameter** (`engine.go:387`).
- `autocascade.Runner` wraps ctx per entry with `automation:<name>`, exactly as
`executeScriptActions` already does.
- Tighten `TestAudit_AC5_TriggeredByOnAutomationCascade` from "non-empty" to the
exact expected label.
- Remove the known-gap section from the audit-log guide.

**Scope — OUT:**

- Any change to the audit record schema, `recordCascade`, or the ctx mechanism —
all already correct.
- Attribution for `set:` actions (they mutate the trigger entity itself, which is
covered by the entity's own update record, not a cascade record).
- Renaming or restructuring `automation.Result` beyond the minimum.
- `cascade:delete-entity:<id>` labels — a different, already-specific label whose
callers must keep it (`recordCascade`'s "callers that already wrapped ctx keep
that label" contract).

**Acceptance Criteria:**

1. **AC1** — a metamodel with two `on: created` automations, both producing
`create_relation`, yields audit records where each relation-create carries
`triggered_by: "automation:<originating-name>"`, not the generic label.
2. **AC2** — same for cascade-created entities (`create_entity` actions).
3. **AC3** — `TestAudit_AC5_TriggeredByOnAutomationCascade` asserts the exact
label `automation:create-checklist-for-req` rather than "non-empty".
4. **AC4** — the generic `"automation"` fallback still applies when no name is
available, and a caller that already wrapped ctx with a specific label (e.g.
`cascade:delete-entity:<id>`) keeps it. No regression in `recordCascade`.
5. **AC5** — the known-gap section is removed from
`docs-project/entities/guides/GUIDE-audit-log.md` and `docs/audit-log.md`
regenerated from it.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — small, and the in-tree pattern is exact.

**Existing Solutions:** the scripted path is the reference implementation of
this very feature, three lines away from where the fix goes:

- `automation.LuaToExecute.AutomationName` (`types.go:149`) — the field to
mirror, with a godoc explaining *why* the name travels ("so that error envelopes
can identify which inline `lua: |` block failed").
- `engine.go:444,455` — set at the engine seam from the in-scope
`automationName` parameter. The two new sites are in the *same function*.
- `runner.go:270` — `audit.WithTriggeredBy(ctx, "automation:"+action.AutomationName)`,
the exact ctx wrap to copy.
- `cascadehost.go:264` — the fallback that already yields to a ctx label.

So this ticket generalizes a working pattern rather than designing one.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **`EntityToCreate` gains `AutomationName string`** — it is already a struct,
so this is one field, mirroring `LuaToExecute`.
2. **Relations need a carrier.** `RelationsToCreate` is `[]*entity.Relation`.
Introduce `RelationToCreate{Relation *entity.Relation; AutomationName string}`
and change the slice's element type. Chosen over a parallel `[]string` because a
parallel slice can silently desynchronise from the one it indexes — length drift
is a runtime bug with no compile-time signal, and this value crosses a package
boundary into autocascade. The struct makes the pairing
unrepresentable-if-wrong.
3. **Engine seam** (`executeAction`, `engine.go:387`): set the name in both
branches. `automationName` is already a parameter — no signature change.
4. **Runner**: `applyRelationCreations` and `processEntityCreations` wrap ctx
per entry with `audit.WithTriggeredBy(ctx, "automation:"+name)` when the name is
non-empty, and pass that ctx to the `host.Write*` / `host.CreateEntity` call.
Leave ctx untouched when empty so AC4's fallback still fires.
5. **Test + docs** per AC3/AC5.

*Where the label is applied matters.* It must wrap the ctx handed to the **host
write call**, not the outer loop ctx — `applyRelationCreations` iterates several
relations that may come from different automations, so a per-loop wrap would
attribute all of them to the last name seen. Per-entry, like
`executeScriptActions` does.

**Files to modify:** `internal/automation/types.go`,
`internal/automation/engine.go`, `internal/autocascade/runner.go`,
`internal/entitymanager/audit_test.go`,
`docs-project/entities/guides/GUIDE-audit-log.md` (+ regenerate `docs/`).

**Alternatives considered:**

1. *Parallel `[]string` of names alongside the relation slice.* Rejected — see
above; desync is a silent runtime bug.
2. *Put the name on `entity.Relation` itself.* Rejected — `entity.Relation` is
the domain type persisted to disk; automation provenance is not part of a
relation's identity and must not leak into storage.
3. *Stamp the label in `cascadeHost` by looking up which automation matched.*
Rejected — the host has no access to that; it is precisely the information the
Result drops, which is why plumbing it is the fix.
4. *Leave it and document.* Rejected — it is already documented as a gap, and
the issue is that the documentation is not the desired end state.

**Dependencies:** none new.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** the automation *name* comes from `schema.yaml`,
which is operator-authored config. CLAUDE.md is explicit that config contents —
including automation names — are not secret, so writing one into an audit record
discloses nothing that the repo does not already.

**Security-Sensitive Operations:** this touches the **audit log**, which is a
security control, so two properties matter:

- *It only ever makes attribution more specific.* No record loses a field; the
generic label remains for the no-name case (AC4). An audit consumer parsing
`triggered_by` sees `automation:<name>` where it previously saw `automation` — a
documented refinement, and the reason AC5 updates the guide.
- *The name is a config identifier, not caller-supplied input*, so there is no
injection surface into the record. Worth noting the label is a plain JSONL
string field, so even a hostile name cannot break the record framing — but
automation names come from the same file that defines the automations, i.e. from
an actor who could simply write whatever records they liked.

Principal attribution is untouched: `recordCascade` keeps taking
`principal.From(ctx)`, and `triggered_by` is orthogonal to *who* acted.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

- **AC1 (the discriminating test)** — two `on: created` automations on the same
entity type, with **different names**, each emitting a `create_relation` to a
different target. Assert each resulting audit record carries its *own*
automation's name. Two automations is the minimum that catches the per-entry
wrap bug: with one, a wrongly-placed per-loop wrap would pass.
- **AC2** — same shape with `create_entity` actions.
- **AC3** — tighten the existing test to the exact label.
- **AC4** — two negative cases: (a) an automation-driven write with no name set
still records `"automation"`; (b) a delete cascade keeps
`cascade:delete-entity:<id>` and is *not* overwritten.
- **AC5** — `docs/audit-log.md` regenerated; assert the gap section is gone and
the file still matches its generated form (the repo's docs check).
- Unit-level: an `internal/automation` engine test asserting `Result` entries
carry the name, so a break is localized rather than only surfacing three
packages away in an audit assertion.

**Edge Cases:**

- An automation with an **empty** `Name:` — the generic fallback must fire, not
`"automation:"` with a dangling colon. Explicitly tested.
- A cascade several levels deep: the entity created by automation A itself
triggers automation B. The record for B's write must say B, not A. Covered by
the queue-driven cascade in the runner; assert it.
- One automation emitting several relations — all carry the same name (trivially
true, but it is what the per-entry wrap must not break).
- `IfExistsReplace` path (`cascadehost.go:178`), which has its own attribution
comment — verify it is unaffected.

**Negative Tests:** empty name → generic label; pre-wrapped ctx → preserved
label. Both above.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Mitigation |
|---|---|
| Per-loop instead of per-entry ctx wrap misattributes to the last automation | The AC1 test uses **two** differently-named automations, which is what makes the bug detectable |
| Changing `RelationsToCreate`'s element type breaks callers | Compiler-enforced; the consumer set is small (engine + runner + tests) |
| An audit consumer parses `triggered_by == "automation"` exactly and breaks | This is the documented intent of the change; AC5 updates the guide. Flag in the PR |
| Silently losing the generic fallback | AC4 tests it explicitly, both branches |

**Effort:** s

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs-project/entities/guides/GUIDE-audit-log.md` — remove the
"Per-automation attribution for cascaded relations / entities" known-gap section
(line ~210). **Edit the entity, not `docs/`**: `docs/audit-log.md` is generated
("This file is auto-generated from docs-project/entities/. Do not edit
directly.") and is regenerated via `just docs`.
- [x] `docs/audit-log.md` — regenerate; verify the gap section is gone and the
`triggered_by` documentation describes the `automation:<name>` form.
- [x] ~~CLI reference / metamodel docs~~ (N/A: no new flag, command, or schema
key — automation names already exist)

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: skipped, and
      with hindsight this was the wrong call — see below.)
- [x] All critical/significant findings addressed in plan — via `/code-review`;
      7 of 9 addressed, 2 deferred with reasons. See the review checklist.

**Design Review Findings:** N/A as a separate pass — but the skip is worth
recording honestly rather than waving through.

The reasoning at the time: the mechanism already existed on the scripted path,
`recordCascade` already preferred a ctx label, and the change was a few lines at
a seam where the needed value was already in scope. That reasoning was sound
about the *plumbing*, which was indeed correct first time.

What it missed is that the plan's own Negative Tests section named the exact
failure a design review would have interrogated — *"pre-wrapped ctx → preserved
label"* — and the plan asserted that an existing test covered it **without
checking which code path that test exercised**. `audit.WithTriggeredBy` being an
unconditional overwrite is a property of the API this change was built on, and
"what happens when two causes are both true?" is a design question, not an
implementation detail. It went unanswered until `/code-review` found the
regression (RR-ZYVERL).

The lesson is narrow and worth carrying: a change can be mechanically trivial and
still carry a *semantic* decision. When a plan writes down a negative test, verify
the named test actually exercises the named path before crediting it.

Code-review findings: RR-ZYVERL (critical); RR-4JUX43, RR-KPTYPY, RR-KA9LD5,
RR-3DWRZX, RR-Z1V5Z6 (significant); RR-14X83F, RR-OPBDAY (minor); RR-2FY0O8
(significant, deferred → BUG-IK9IYL).
