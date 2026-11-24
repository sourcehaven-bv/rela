---
id: PLAN-SYKHOZH
type: planning-checklist
title: 'Planning: List toggle cells: widget toggle with on/off values'
started: "2026-11-24"
completed: "2026-11-24"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: `widget: toggle` with `on:`, `off:`, `other:` on
`lists.*.columns`; `_toggles` list row extra for reads that name the list
(`list_id`); SPA checkbox cell that PATCHes the other value; todo-app done
checkbox (and a reopen transition). Out: toggles on dashboard cards, view
sections and kanban; bulk toggle; trigger `unset`; computed grouping.

**Acceptance Criteria:**
1. Load fails for: toggle on a non-property column, on a property that is not enum or bool, `on`/`off` missing, equal, or not a value of the enum, an enum with other values and no `other:`, an unknown `other:` value, `other:` on a bool or on an enum with only two values, toggle keys on dashboard or section columns (validator table test).
2. `GET /api/v1/todos?list_id=inbox` sets `_toggles.status` per row: true when the reader may write the flip (field writable, target option allowed, transition allowed); false otherwise (handler test).
3. Without `list_id`, or with a list that has no toggle column, rows carry no `_toggles` (handler test).
4. A property the reader cannot see gets no entry; a name that is not a property of the type is ignored (handler test).
5. The toggle extra adds no per-row store reads: budget test with 5 and 50 rows.
6. SPA: clicking the cell PATCHes `{status: on|off}`; a row with `_toggles.status=false` renders disabled; `other: hidden` renders no checkbox; `other: on` renders checked (vitest).

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: shape fixed in the parent spec)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: config + UI in existing stack)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: in-house pattern)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- Row extras `?busy=` (busy_rows.go) and `?rollups=` (rollups.go) through `serveListRowExtras`.
- `computeTransitions` and `fieldVerdicts` in affordances.go; `primeVerdicts` + `primeFlowRuns` for batching.
- `refuseListOnlyColumn` (busy_column.go) keeps list-only keys off other columns.
- `render: input` editable table cells (EditableRowCells.vue) for the PATCH write.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** `ListColumn` gains `Widget`, `On`, `Off`, `Other`
(strings; bool on/off written `true`/`false`). Validation in
`validate_toggle.go`, called from the list column loop; `refuseListOnlyColumn`
refuses the keys elsewhere. Wire: `Activity.Toggles map[string]any`
(`_toggles`): per toggle property, the target value the reader may write now
(JSON bool for a boolean); absent when the flip is not allowed or the cell has
no checkbox. The SPA PATCHes the sent value, so the flip rule lives only on the
server. `serveRowToggles` (toggle_rows.go) runs when the read names a list
(`list_id`, already sent by the SPA) whose columns include toggles of the row
type. On/off come from config, so the client cannot choose the target. It primes
verdicts and flow runs for the page once, and per row computes the current value
(an unset machine property reads as the machine entry value, as the write path
does; an unset bool or enum reads as off) and the target (off when current is
on, or is other with `other: on`; on when current is off; none for other with
disabled/hidden) and whether the reader may write it: field visible and
writable, target option allowed, and for a machine-typed property an allowed
transition current→target. Transition `when:` traversals are bound once for the
page: a new `Transitions.PrimeTraversals(ctx, type, ids)` carries one `Bind`
over all page ids on ctx, and `bindTraversals` uses it when present. Writes go
through the normal PATCH path, which enforces ACL and transitions; `_toggles` is
UX only. Alternative rejected: reuse `_fields`/`_transitions` on list rows; it
sends every verdict of every row and puts the flip rule in the SPA.

**Files to modify:**
internal/dataentryconfig/{config.go,validate.go,validate_toggle.go,busy_column.go},
internal/configedit/allowlist.go, internal/apiwire/v1/responses.go,
internal/dataentry/{comment_counts.go,toggle_rows.go}, frontend list cell +
types, examples/todo-app/{data-entry.yaml,schema.yaml}, docs-project guide.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Config (operator, trusted, validated at load).
Query `list_id`: an unknown list or a toggle column of another type yields no
`_toggles`, no error. PATCH body: existing write path.

**Security-Sensitive Operations (threat model):**
- Value disclosure: `_toggles` for a property hidden by `visible:` would reveal its current value (the flip target). Mitigation: no entry when the field is not visible; rows are already row-gated.
- Privilege escalation: the toggle writes via PATCH, which runs AuthorizeUpdate, field grants, option grants and transition guards; `_toggles` grants nothing.
- Cost/DoS: one prime per page; budget test pins it. Toggle columns per list are bounded by config.
- Config is not secret: the list name in the query is fine.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** AC1 validator table test; AC2-4 handler tests in dataentry
with an ACL policy; AC5 querybudget test; AC6 vitest on the cell.

**Edge Cases:** absent value counts as `off`; bool stored as string or bool;
value outside on/off with each `other:` mode; read-only flow run; denied option.

**Negative Tests:** every load failure in AC1; unknown `list_id`.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Transition verdict cost per row (mitigated by the page-level
transition prime; the budget fixture uses a `related()` when). A list
`condition:` on the toggled property drops the row after the write; the SPA
reloads like move actions. Plimsoll field cap on ListColumn (check with `just
plimsoll`). Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** docs-project GUIDE-data-entry Column Options; API
reference `_toggles`.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-CIQPCHD, RR-WTXZKJR (significant, addressed);
RR-113DCSJ, RR-MQXAHJ2, RR-P55RKBV, RR-1SBMYMW (minor, addressed in plan).
