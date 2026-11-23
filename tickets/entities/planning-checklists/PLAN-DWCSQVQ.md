---
id: PLAN-DWCSQVQ
type: planning-checklist
title: 'Planning: Flows: step form from a data-entry form instead of writes:'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope:

- `form:` on a state names a form in `data-entry.yaml`. `writes:` is removed
with no migration and no tombstone error; nobody uses it yet, so the strict
decoder's "field writes not found" error is enough.
- The step form in the SPA renders the named form's fields with their full
field config (widget, label, placeholder, help, hint, span, transitions,
`visible_when`, `required`, `required_when`).
- The server takes the step's accepted properties from the form's fields.
- The trigger ACL check (RR-25REQKG) locks the form's fields instead of the
`writes:` list.
- `rela flow validate` checks the form and action references.

Not in scope (refused with a clear message, follow-up if wanted):

- Forms with `relations:`, `steps:` (wizard) or `body: true`. A step writes
properties only; writing relations or the body needs its own write path.
- Server-side evaluation of `required_when` / `visible_when`. Normal form saves
do not evaluate them in Go either (`dataentryconfig.FormField` godoc).

**Acceptance Criteria:**

1. A state with `form: x` where data-entry.yaml declares form `x` for the
subject type loads; the run view carries `waiting.form` and `waiting.fields`.
Test: appbuild step test reads the run view.
2. A step that sends a property not in the form's fields answers invalid
(422). Test: `TestFlowControl_CompleteStep` "property outside the form".
3. A form that does not exist, is for another type, has relations, steps or a
body, or names a derived/reserved/undeclared property, refuses the engine build
and `rela flow validate`. Test: table test on `checkFlowForms`.
4. `form:` on a final state, a state without `waiting_for:` or a job state is
refused by flowcfg. Test: `load_test.go` table.
5. A trigger that sets a property the step form edits is refused in Configure.
Test: triggeracl "a property a flow locks" with a form.
6. The SPA step form renders the form's widget, label and help, hides a field
whose `visible_when` is false, and blocks submit on a missing required field.
Test: FlowRunCard / FlowStepForm vitest.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: the design follows the existing flow `action:` reference to data-entry.yaml)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: internal config wiring)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal config wiring)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- `internal/appbuild/flowjobs.go` `checkFlowActions` / `loadActions`: a job
step's `action:` names a data-entry.yaml action, checked when the engine is
built. `form:` follows the same pattern.
- `frontend/src/composables/useFormWizard.ts`: evaluates `visible_when` and
`required`/`required_when` for flat and wizard forms; reused by the step form
with `syncUrl: false`.
- `FieldRenderer.vue` already renders every `FormField` option.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

- flowcfg: `rawState.Form string`; `State.Form()`; name check; only on a
non-final state with `waiting_for:`; job states refuse it. Drop `writes`,
`MaxWrites`, `State.Writes()`.
- flowengine: `Explanation.Form` replaces `Writes`.
- appbuild: `stepForms` (form name -> accepted properties) built from
data-entry.yaml by `resolveStepForms`, which also checks the references. Built
in `buildFlowEngine` next to `checkFlowActions`, kept on `Services`, handed to
`FlowControl`. `inputNames` uses it; `waitingOf` fills `flowview.Waiting{Form,
Fields}`.
- Exported `CheckFlowDataEntry` (actions + forms) for `rela flow validate`.
- triggeracl: `configedit.TriggerEdit` gains `DataEntryBefore/After`; the
judge locks the form fields. A data-entry change re-judges unchanged triggers
like a schema change does.
- SPA: `FlowStepForm` reads the form from the schema store
(`getForm(run.waiting.form)`), renders its fields through `FieldRenderer` with
`useFormWizard` for conditions, prunes hidden fields, validates required fields
before submit.

**Alternatives considered:**

- Inline field objects under `writes:`: rejected by the user; it duplicates
the form model in a second file.
- Resolve the form inside flowcfg: rejected; flowcfg loads flows.yaml alone
and would need data-entry.yaml, as the action check already avoids.
- Support relations in step forms now: rejected for scope; needs a relation
write path for steps.

**Files to modify:** `internal/flowcfg/{raw,states,compiled,job}.go`,
`internal/flowengine/person.go`, `internal/flowview/flowview.go`,
`internal/appbuild/{flowjobs,flowruntime,flowcontrol,flowsteps,triggeracl,appbuild}.go`,
`internal/configedit/triggers.go`, `cmd/rela-server/configure.go`,
`internal/cli/flows.go`, `internal/mcp/tools_flows.go`,
`frontend/src/components/flows/FlowStepForm.vue`, `frontend/src/api/flows.ts`,
tests and fixtures, `docs-project/entities/guides/GUIDE-flows.md`,
`e2e/tests/flows-project.ts`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- Step input (HTTP/MCP `properties`): allowlisted against the form's
non-hidden field properties; anything else is 422 before any read of the
subject. The write still goes through `AuthorizeUpdate` and the field gate.
- `form:` and data-entry.yaml: operator config, checked at load (exists,
subject type, no relations/steps/body, declared non-derived non-reserved
properties).

**Security-Sensitive Operations:**

- The trigger ACL lock (RR-25REQKG) must keep refusing triggers that write
step properties; it now derives them from data-entry.yaml, so Configure passes
both versions of that file and re-judges on a data-entry change.
- Config names (form names) are not secret; error messages may name them.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see the acceptance criteria; AC 1, 2 and 5 run against a
real app built on disk (`appbuildOnDisk`); AC 6 in vitest; the e2e flow project
switches to `form:`.

**Edge Cases:**

- A form field with `hidden: true`: not accepted, not rendered.
- A form with no fields: the step completes with no properties.
- data-entry.yaml absent while a flow names a form: build refused.
- A run on an older flow version naming a form that no longer exists: no
accepted fields; a step with properties is 422.

**Negative Tests:** unknown form, wrong entity type, relations, steps, body,
derived property, property outside the form in a step.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Mitigation |
| ---- | ---------- |
| A data-entry.yaml edit removes a form a flow names and the reload fails. | Same behaviour as an action reference today; the error names flow, state and form. |
| Trigger lock silently loses the step properties. | triggeracl test with a form-locked property. |

**Effort:** m

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `GUIDE-flows` (docs/flows.md): `writes:` -> `form:`, step form section,
422 wording, HTTP payload.
- [x] `GUIDE-mcp-server` if it names `writes`.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: the user set the direction in conversation, dropping `writes:` for a data-entry form, and asked to proceed; `/code-review` runs in review)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** none recorded
