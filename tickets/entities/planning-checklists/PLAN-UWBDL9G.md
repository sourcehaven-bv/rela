---
id: PLAN-UWBDL9G
type: planning-checklist
title: 'Planning: Declarative metric capture: schedule task that appends a dashboard card value as a record'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

A schedule task captures one dashboard card's figure under the task's `run_as`
identity and appends it as one record row, with no Lua.

```yaml
tasks:
  - name: control-coverage
    every: monday
    run_as: system:metrics
    capture:
      dashboard: default        # optional; a name under dashboards:, or default
      card: Control coverage    # the card's title, unique on that dashboard
      record: measurement       # a record type
      owner: MET-control-coverage
      field: value              # integer or decimal field of the record type
```

**Scope:**
- IN: `capture:` on a task, exclusive with `script:` and `template:`; refused with `for_each:` and `capabilities:`; `run_as` is required (RR-PKTE9FY).
- IN: capturable cards are `list:` cards on a dashboard without `entity_type`, with display `count` (rows), `ratio` (matched / total x 100), `sum`, `avg`, `min`, `max` (numeric property only).
- IN: schema checks at load (record type exists, not a ledger, field is integer/decimal, no other required field).
- IN: card checks at run time (the data-entry config hot-reloads separately from schedules.yaml).
- IN: rounding to the field: integer rounds half away from zero; decimal uses the field's `scale`, or 2 places when it declares none.
- IN: rela-server and rela-desktop run capture tasks. `rela scheduler` refuses to start when a task has `capture:`: it has no data-entry app to compute cards.
- OUT: `query:` cards (computed by the SPA through search, no server value), table/breakdown/matrix/line cards, entity-dashboard cards (no anchor), a free-standing query in schedules.yaml (would duplicate card semantics; can follow later).

**Acceptance Criteria:**
1. A capture task over a count card appends one row whose field holds the row count `run_as` can read.
2. A ratio card stores the percentage, rounded to the field's scale.
3. sum/avg/min/max cards store the folded value; a date min/max is refused.
4. The figure is computed under `run_as`: rows the identity cannot read are not counted.
5. A failed evaluation (unknown dashboard/card, ambiguous title, unsupported display, partial value, no value, ratio over 0 rows, append refused) stores nothing, fails the run with a message naming the task, and enters the normal retry ladder.
6. Config errors (capture with script, with for_each, with capabilities, missing keys, bad record/field) fail schedules.yaml loading.
7. `rela scheduler` refuses a config with a capture task, with a message saying to run it in rela-server or rela-desktop.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: follow-up of TKT-RK810V7, approach follows existing scheduler job kinds)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal composition of existing card evaluation and record append)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- Card evaluation: `dataentryconfig` card config, `internal/dataentry/dashboard_card.go` `dashboardCardValue` (gate, then redact, then fold).
- Record append: `lua.RecordAppender` (`internal/lua/records.go`), supplied by `appbuild.LuaRecords`; it is already in `ScheduledLuaWriteDeps().Records`.
- Job kind pattern: subscription tasks (`internal/scheduler/subscriptions.go`) read the current config at run time and fail "not configured" for a removed task.
- Request edge outside HTTP: `attachBoundReadGate` (webhooks), default world binding as in `nextActionSourceWorld`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
1. `scheduler.TaskConfig.Capture *CaptureConfig` with keys dashboard, card, record, owner, field. Validation in `validateTask`; schema checks in `Config.ValidateMetamodel`.
2. Consumer-side interface in scheduler: `CardEvaluator { CardFigure(ctx, dashboard, card string) (*big.Rat, error) }`. Supplied as a functional option `WithCards` to `New`/`NewWithQueue`/`StartBackground` (RR-EH5IDUD). Nil: capture unsupported; the capture job kind is not registered and capture tasks are skipped with a log at load and on reload (RR-U55X4FQ).
3. New job kind `scheduler/capture` with task name and run id. The handler loads the current config, finds the task, builds the task context (`taskContext`: stamps run_as, binds identity), calls the evaluator, rounds once to the field, appends one row stamped with the run's read time (RR-8SHEV02) through `ws.ScheduledLuaWriteDeps().Records`, records the outcome via `finish`.
4. dataentry: `dataentry.CardFigures(a *App)` returns a small type with `CardFigure` (RR-V2EYZU8), in `dashboard_capture.go`. It attaches the bound read gate under withACLEnforced and binds the default world through the world lookup, failing when it misses (RR-L4ZE9SC) (the operation has no HTTP middleware), resolves the dashboard and the card by title, refuses unsupported cards, calls `dashboardCardValue(ctx, a, s, card, nil)`, refuses a partial value on sum/avg/min/max only (RR-7I2WN08), refuses a missing value, and returns the exact figure from a shared exact fold, never from display text (RR-76AWXKG). min/max are numeric only by declared property type and convert through metamodel.ParseDecimal (RR-A1W117I). It ignores the card's `permission:` (a UX filter, not a boundary).
5. Wiring: rela-server and rela-desktop call `UseCards` with the generation's dataentry App. `StartBackground` gets a variant taking the evaluator (or an options arg). `rela scheduler` refuses capture tasks.
6. Docs: scheduled-tasks and records guides in docs-project, regenerated docs.

**Alternatives rejected:**
- A query spec in schedules.yaml evaluated by the scheduler: duplicates list filters, query scopes and conditions, and the stored series would drift from the card the dashboard shows.
- Moving card evaluation out of dataentry into a shared package: large refactor of the list pipeline, out of proportion.
- Running capture in `rela scheduler` by building a dataentry App there: `cli` may not depend on `dataentry`.

**Files to modify:**
- internal/scheduler/config.go, capture.go (new), jobs.go, scheduler.go (+ tests)
- internal/dataentry/dashboard_capture.go (new, + test)
- cmd/rela-server/serve.go, cmd/rela-desktop/main.go, internal/cli/scheduler.go
- docs-project guides: scheduled-tasks, records; regenerated docs/

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- schedules.yaml (operator-authored): KnownFields decoding, allowlist of capturable displays, schema checks at load.
- data-entry.yaml card: resolved at run time; unknown/ambiguous names fail the run.

**Security-Sensitive Operations:**
- Reads: run under the bound `run_as` Request through the same gated list pipeline as the dashboard (gate before fold). No elevated reader.
- Write: the record service checks update access on every face of the owner, as for `rela.append_records`.
- Disclosure: the stored figure is an aggregate cached across principals: whoever reads the owner reads it. This is a deliberate exception to the per-principal aggregate rule; run_as is required and the guide says the owner's read rule must be no wider than run_as.
- A partial value (some property hidden from run_as) is refused rather than stored, so a figure never silently covers less than it claims.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
- AC1-4: dataentry `CardFigure` table test over a test App with ACL: count, ratio, sum, avg, min/max, and a role that reads a subset (figure counts only readable rows).
- AC5: scheduler capture job test with a fake evaluator and fake appender: success appends one row with the rounded value; each failure records a failed run and appends nothing.
- AC6: config_test table cases.
- AC7: CLI check function test.

**Edge Cases:**
- Ratio over 0 rows: refused. Count of 0: stored as 0.
- Duplicate card titles: refused as ambiguous.
- Decimal field without scale: 2 places. Negative values round away from zero.
- Task removed between enqueue and run: fails as not configured.

**Negative Tests:** unknown dashboard, unknown card, query card, table card,
entity dashboard card, date max, partial value, nil evaluator, append error.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Card titles are not stable ids; renaming a card breaks the task. Mitigation: run fails loudly with the title it looked for.
- Data-entry config and schedules.yaml are validated separately; a card error shows up only at run time. Mitigation: clear error, retry ladder, logs.
- Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- GUIDE-scheduled-tasks (capture section), GUIDE-records (trend chart pointer); regenerated docs/.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-U55X4FQ, RR-EH5IDUD, RR-PKTE9FY, RR-76AWXKG
(significant); RR-A1W117I, RR-7I2WN08, RR-8SHEV02, RR-L4ZE9SC (minor);
RR-V2EYZU8 (nit). All addressed in the plan.
