---
id: PLAN-YGNLZH
type: planning-checklist
title: 'Planning: Analysis view: reveal which detail failed a validation (missing headers etc.) on hover/click'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN:
- Surface *which required (exact-match) headers are missing* for content-rule (`content.required-headers`) violations in the data-entry **Analysis view** Validations table.
- **Split the row's click targets** (per user decision): the **entity-title cell** navigates to the entity; the **message cell** toggles a full-width **detail row** listing the missing headers. The row is no longer a single click target. See RR-7UJUAI.
- Thread the detail from the content-rule checker up through the validation → validator → analyze → API → SPA chain.

OUT (explicit scope boundaries):
- **Pattern-based header checks** (`IsPattern()`) — excluded from the detail, because `GetMatchString()` returns a raw regex that is misleading to show a user (RR-ZOGG1X). Only exact `Header` misses are reported.
- **CLI `analyze`/`validate`** — those consume the *separate* `validator.Violation` struct via `CheckAll`, which does NOT gain `Detail` under this plan (RR-1GP8NI). Data-entry Analysis view only.
- Checklist-rule / property-rule / cardinality detail; Lua-rule messages (already dynamic).
- Write-path/autosave 422 surface (that is FEAT-13863O's other ticket TKT-1ARGYN).

**Acceptance Criteria:**
1. Given an entity failing a `content.required-headers` rule with exact headers `## Beschrijving` and `## Oorzaak` both missing, the Analysis view MESSAGE cell shows the rule description; clicking it expands a full-width detail row listing the two missing headers by name; clicking again collapses it. Test: unit test on `MissingRequiredHeaders` asserts it returns exactly the two match-strings; component test on `AnalyzeView.vue` asserts click toggles the detail row and it contains the headers.
2. Clicking the **entity-title** cell navigates to the entity (`/entity/:type/:id`) — unchanged destination, now scoped to the title cell rather than the whole row. Test: component test asserts title-cell click routes; message-cell click does NOT route.
3. Satisfied entity → no issue row (unchanged). Existing analyze tests stay green.
4. Lua-script-failure row → clicking the message cell opens the existing `ScriptErrorDialog` (the script-error detail moves to the message cell alongside missing-header detail — the message cell owns all "why did this fire" reveal). Title cell inert (no entity). Existing script-error tests updated for the new trigger location.
5. Non-content (then-filter / Lua) violation → no `detail` on the wire (omitempty), message cell has no expand affordance (no detail row). Test asserts absent `detail`.
6. A `pattern:` header miss → NOT listed in detail (RR-ZOGG1X). Test asserts pattern checks are excluded.

## Research

- [x] For larger features: run `/research` — N/A (m-effort, well-scoped, single reported case)
- [x] Searched for existing libraries — N/A (internal plumbing + existing Vue)
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — effort m, approach is clear from the code.

**Existing Solutions / prior art in-codebase:**
- **`scriptError` envelope** (`ScriptErrorEnvelope`, `AnalysisIssue.ScriptError`, `AnalyzeIssue.scriptError`, `ScriptErrorDialog` opened from `AnalyzeView.onIssueClick`) is the precedent for carrying structured per-issue detail end-to-end and click-to-reveal. This ticket adds a *lighter* per-issue detail (a string list rendered in an inline detail row).
- **Lua violations already carry a dynamic per-entity message** (`LuaViolation.Message` → `Violation.Description` via `runLuaForEntity`, validation.go:337-346). `newViolation(rule, e, description)` already treats `description` as an override slot.
- **The missing-header identity is already computed and discarded**: `CheckContentRule` (`content_rule.go:17-21`) loops `rule.RequiredHeaders` and returns `false` on the first miss.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified
- [x] Design-review findings folded in (RR-UFUX7T, RR-ZOGG1X, RR-M2880C, RR-7UJUAI, RR-WFJQ1I, RR-1GP8NI, RR-WJ65QK)

**Technical Approach:**

Add an optional structured `Detail []string` to a violation and thread it
through the two lossy narrowing points: (a) `CheckContentRule` returns only
`bool`; (b) `validator.RuleResult.Violations` flattens `[]Violation` to
`[]string` of IDs and `analyzeValidations` re-derives the message from
`rule.Description`.

1. **`internal/validation/content_rule.go`** — add `MissingRequiredHeaders(content string, rule *metamodel.ContentRule) []string`:
   - Iterate `rule.RequiredHeaders`; for each `hc`, **skip if `hc.IsPattern()`** (RR-ZOGG1X) and **skip if `hc.GetMatchString() == ""`** (RR-M2880C — mirrors the primitives' trivial-pass); otherwise if `!matchHeaderCheck(headers, hc)` collect `hc.GetMatchString()` (the exact `## Header` string, which reads naturally — RR-WJ65QK).
   - Collect **all** misses (no short-circuit).
   - Return **`nil`** (not `[]string{}`) when nothing is missing (RR-WJ65QK).
   - Keep `CheckContentRule` behaviorally identical: it still short-circuits on the first miss for exact AND pattern AND checklist (do NOT re-route it through `MissingRequiredHeaders`, which ignores patterns/checklist). `MissingRequiredHeaders` is a *detail-only* helper, not the pass/fail authority.

2. **`internal/validation/validation.go`** — add `Detail []string` to the `Violation` struct. In `checkEntityAgainstRule`'s content branch (line 303-305): when `!CheckContentRule(...)`, compute `missing := MissingRequiredHeaders(e.Content, rule.Content)` and attach it, **only when `len(missing) > 0`** (RR-WJ65QK). Base `Description` stays `rule.Description`. Lua/then-filter call sites keep `Detail == nil` (RR-WFJQ1I: detail is per-*violation*, empty for non-content).

3. **`internal/validator/validator.go`** — change `RuleResult.Violations` from `[]string` to a small exported struct, e.g. `type RuleViolation struct { EntityID string; Detail []string }`. `CheckRuleFull` populates `EntityID` + `Detail` from each `validation.Violation`. Keep `CheckRule` (public `[]string` method used by `mcp/tools_analysis.go:252`) as a wrapper mapping to `EntityID`s — its callers and tests are untouched.

4. **`internal/dataentry/analyze.go`** — `AnalysisIssue` gains `Detail []string`. **Rewrite the loop at line 448** (RR-UFUX7T — this is the critical one): `for _, v := range full.Violations { e, err := st.GetEntity(ctx, v.EntityID); ...; Message: rule.Description, Detail: v.Detail }`. The loop var is now the struct, not a bare `id string`. Add a test asserting `AnalysisIssue.Detail` is populated (guards the no-op regression).

5. **Wire** — `APIIssue` (`settings_handlers.go:26-41`) gains `Detail []string` with `json:"detail,omitempty"`; `handleV1Analyze` mapping (`api_v1.go` ~1910-1932) copies it; `visibleAnalysisIssues` passthrough unaffected. omitempty drops both nil and empty → absent on the wire.

6. **Frontend** — `AnalyzeIssue` (`config.ts:271-285`) gains `detail?: string[]`. Rework `AnalyzeView.vue` from row-wide click to **split cell targets** (RR-7UJUAI):
   - **Remove** the row-level `@click`/`@keydown`/`tabindex`/`clickable` on `<tr>` (lines ~220-225). The row is no longer interactive.
   - **Entity-title cell** (line ~227-235): make the title (`.entity-title`) a `role="button"` with `tabindex="0"`, `@click`/`@keydown.enter`/`@keydown.space.prevent` → a new `onEntityClick(issue)` that does the `router.push('/entity/:type/:id')` (only when `entityId && entityType`).
   - **Message cell** (line 240): make the message text a `role="button"` with `tabindex="0"`, `aria-expanded`, `aria-controls`, `@click`/keyboard → `onMessageClick(issue, idx)`. If `issue.scriptError` → open `ScriptErrorDialog` (move that branch here). Else if `issue.detail?.length` → toggle a per-row `expanded` state (a `Set<string>` or `ref` keyed by the row's stable key). If neither, the message is plain text (no button role).
   - **Detail row**: wrap the `v-for` body in `<template v-for>` so each issue emits (a) the `<tr class="issue-row">` and (b) a second `<tr class="issue-detail-row" v-if="isExpanded(key)">` with a single `<td colspan="4">` holding a `<ul>` of `issue.detail` (each `## Header` as an escaped `{{ }}` list item — **no `v-html`**, RR-WJ65QK). Give both `<tr>`s distinct `:key`s (`key` and `key + '-detail'`).
   - Scoped CSS for `.issue-detail-row` (muted background, small "Missing required headers:" label + the list). Full table width, not squished into the message cell (per user decision).

**Alternatives considered:**
- *Row-wide click + native `title` tooltip* (earlier plan/RR-7UJUAI) — superseded by user request: split the click targets instead. Better because (a) each target is independently focusable → clean keyboard a11y with no `@click.stop` propagation dance, and (b) a persistent expanded detail row beats an ephemeral hover tooltip for scanning/copying header names.
- *Expand inside the message cell* (detail `<ul>` inside the `<td>`) — rejected per user: leaves the detail squished under one column with dead space beside it. The full-width `colspan` detail row reads cleanly.
- *Rewrite the NCR rule as Lua* — rejected: abandons the declarative `content.required-headers` form and hides the logic in per-project Lua.
- *Bake the header list into the flat `Description` string* — rejected: mixes static + computed text in one untyped string, harder to style, and loses the detail-on-demand UX.

**Files to modify:**
- `internal/validation/content_rule.go` (+ `content_rule_test.go`)
- `internal/validation/validation.go` (+ tests)
- `internal/validator/validator.go` (+ tests)
- `internal/dataentry/analyze.go` (+ `analyze_test.go`)
- `internal/dataentry/settings_handlers.go`, `internal/dataentry/api_v1.go` (+ handler test asserting `detail` passthrough)
- `frontend/src/types/config.ts`, `frontend/src/views/AnalyzeView.vue` (+ component test)

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- `detail` values are exact header match-strings from the **metamodel** (`rule.RequiredHeaders[].Header`), not user free-text or file paths. Rendered via Vue text interpolation in the detail `<ul>` (auto-escaped); no `v-html` (RR-WJ65QK).
- ACL: detail rides inside `AnalysisIssue`, already ACL-filtered by `visibleAnalysisIssues` before serialization. No new visibility surface.

**Security-Sensitive Operations:** none new.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified
- [x] Negative test cases defined
- [x] Integration test approach defined

**Test Scenarios:**
- AC1: `content_rule_test.go` table test on `MissingRequiredHeaders` (all-present→nil; one missing→that one; multiple→all). `analyze_test.go`: rule + fixture entity → `AnalysisIssue.Detail` populated (guards RR-UFUX7T). `AnalyzeView` component test: message-cell click toggles the detail `<tr>`; the detail row lists the headers; a second click collapses.
- AC2: component test — title-cell click calls `router.push`; message-cell click does NOT navigate.
- AC3: existing analyze tests green.
- AC4: script-error row — message-cell click opens `ScriptErrorDialog` (updated trigger); title cell inert.
- AC5: then-filter/property violation → `Detail` nil → API omits `detail` → message cell is plain text, no detail row.
- AC6: `pattern:` header check → excluded from `MissingRequiredHeaders` output (RR-ZOGG1X).

**Edge Cases:**
- Empty match-string `HeaderCheck{}` → skipped, not surfaced as a header named "" (RR-M2880C).
- Pattern header → excluded (RR-ZOGG1X).
- Rule with both header + checklist failing → header detail only; no panic.
- Rule with `lua:` + `content:` → Lua short-circuits; content detail not reached (RR-WFJQ1I) — expected, documented.
- Multiple violations same EntityID (Lua) → each is its own row; Lua ones have nil detail (no expand).
- Expand state keyed by the row's stable key → survives re-render; two rows expanded independently.
- `MissingRequiredHeaders(content, nil)` → nil, no panic.
- git-crypt unreadable body → entity skipped upstream (unchanged).
- Long missing-header list → full-width detail row wraps; no horizontal overflow.

**Negative Tests:**
- API round-trip with no detail → JSON omits `detail`; SPA `issue.detail?.length` falsy → no button role, no detail row.

**Integration approach:** `analyze_test.go` exercises store → validator →
analyze → `AnalysisIssue` with a real fixture + real rule. Wire mapping covered
by extending an existing `handleV1Analyze` handler test to assert `detail`
passthrough.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed
- [x] Effort estimated

**Risks:**
- *Silent no-op if analyze.go loop not rewritten* (RR-UFUX7T) — mitigated by the explicit step 4 + a `Detail`-populated assertion test.
- *`RuleResult.Violations` type change* — blast radius verified small: only `analyze.go:448` consumes `full.Violations`; `CheckRule` `[]string` wrapper covers `mcp/tools_analysis.go`; CLI/analysis use the separate `validator.Violation`/`CheckAll` (RR-1GP8NI).
- *Row → split-cell click rework* is the largest frontend change. Moving from one row handler to two cell handlers + a `<template v-for>` detail row must preserve: script-error dialog trigger (now on message cell), entity nav (now on title cell), keyboard operability of both targets, and the `:key` split. Mitigated by the AC2/AC4 component tests and running the app.

**Effort:** m — mechanical backend threading across ~6 files, one genuine type
change (`RuleResult.Violations`), plus a moderate `AnalyzeView.vue` rework
(split targets + detail row).

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] `docs/data-entry.md` (or analyze-view section, if present) — note content-rule violations now show which required headers are missing (click the message to expand), and that the entity title (not the whole row) navigates. Confirm exact file during implementation; N/A if no analyze-view doc exists.
- [x] N/A for docs/metamodel.md, cli-reference.md, CLAUDE.md, README.md — no schema/command/convention change.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings (all folded into Approach above):**
- RR-UFUX7T (critical) — analyze.go loop rewrite. Addressed: step 4.
- RR-ZOGG1X (significant) — exclude pattern checks. Addressed: step 1 + scope boundary + AC6.
- RR-M2880C (significant) — empty-match-string skip. Addressed: step 1.
- RR-7UJUAI (significant) — click-target a11y. Addressed via user-directed **split cell targets** (title navigates, message toggles a detail row); each target independently focusable, no `@click.stop`. Addressed: step 6.
- RR-WFJQ1I (minor) — detail is per-violation/empty for non-content. Addressed: step 2 + edge cases.
- RR-1GP8NI (minor) — CLI scope boundary. Addressed: scope OUT.
- RR-WJ65QK (nit) — nil not empty slice, escaped render. Addressed: steps 1/2/6.
