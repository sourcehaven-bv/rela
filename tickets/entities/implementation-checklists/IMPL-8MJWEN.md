---
id: IMPL-8MJWEN
type: implementation-checklist
title: 'Implementation: Inline entity creation from relation fields in the data-entry edit form'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

**What was built:**

| Layer | Change |
| --- | --- |
| `internal/apiwire/v1/responses.go` | `SidebarResponse.InlineCreate` — entity type → resolved form id |
| `internal/dataentry/views_handler.go` | `inlineCreateForms(ctx)`: a type is listed only when `createFormForType` resolves AND `computeCollectionActions` permits `create` |
| `internal/dataentryconfig/{config,validate}.go` | deleted `FormRelation.AllowCreate` / `.CreateForm` + the now-dead validation branch |
| `frontend/src/composables/useInlineCreate.ts` | offer resolution + the structural depth cap |
| `frontend/src/composables/useFormWizard.ts` | `syncUrl` option so a nested wizard leaves `?step=` alone |
| `frontend/src/components/forms/DynamicForm.vue` | `embedded` prop + `inline-created` / `inline-cancelled` emits; 7 page-scope side effects guarded |
| `frontend/src/components/forms/InlineCreateFormModal.vue` | new dialog hosting an embedded `DynamicForm` under `v-if` |
| `RelationPicker.vue` / `RelationCards.vue` | the affordance in both widgets |
| `InlineCreateModal.vue` | **deleted** (config-bypassing) |

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

**Automated coverage added:** 5 Go handler tests (`inline_create_test.go`) + 1
resolver-preference test; 6 depth-cap tests (`useInlineCreate.test.ts`); 7
widget tests (`InlineCreate.widgets.test.ts`); 11 embedded-mode tests
(`DynamicForm.embedded.test.ts`); 6 e2e tests replacing the vacuous placeholder.

The Go tests were **mutation-checked**: stubbing out the handler wiring fails 4
of the 5, so they have teeth rather than passing vacuously (which is exactly how
the old e2e placeholder failed).

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Ran `rela-server` against `prototypes/data-entry/project` (whose `acl.yaml` has
a real editor/viewer split), driving the browser.

*Permission gate (AC1/AC2) — at the API:*

- `RELA_DATAENTRY_USER=alice@example.com` (editor, `create: ["*"]`) →
`inline_create` lists all 7 types with forms (`category: create_category`,
`ticket: create_ticket`, …).
- `RELA_DATAENTRY_USER=bob@example.com` (viewer, read-only) → the field is
**absent entirely**. Same server, same config: only the principal differs, which
also demonstrates the payload is principal-scoped.

*Full round trip (AC4/AC5/AC6) — in the browser:*

1. Opened `/form/create_ticket`, typed "Manual verification draft", left
priority at `medium`, body template intact.
2. The `belongs to` picker showed **"+ New Category"** — with `allow_create` /
`create_form` deleted from the prototype config, so the offer is genuinely
derived from permission + a registered form.
3. The modal rendered the **configured** `create_category` form: the authored
label "Category Name", its placeholder ("e.g., Backend, Frontend, DevOps"), the
`color` field's help text, and a markdown body — i.e. the form definition, not
the raw metamodel dump the deleted modal produced.
4. Created "Documentation" → modal closed, `Documentation (CAT-001)` appeared
selected in the picker, and **every host field was still populated**. No
navigation occurred.
5. Submitted the host form. `TKT-007` persisted with
`relations: {"belongs-to": ["CAT-001"]}`.
6. `CAT-001` persisted as `{name: "Documentation", status: "draft"}` — note
`status` came from the configured form's default, which the old modal (which
stripped falsy/unset values) would have dropped.

Test data removed afterwards; `git status` on `prototypes/` shows only the
intended `data-entry.yaml` edit.

*Edge cases verified by automated test rather than by hand:* the depth cap
(AC10, e2e + unit), create-without-read (Go), the cards link-step routing and
its "Created … — link it below" notice, and the incoming-direction picker
(BUG-10IPBP's `reverse-relations.spec.ts` still passes).

**A real bug this caught:** the first e2e run failed because the picker never
closed the modal after a create — the deleted modal had emitted `close` itself,
mine does not. Fixed in `handleEntityCreated`, and the widget unit test now
asserts `show === false` so it cannot regress.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — the offer rule lives in ONE composable
consumed by both widgets, and `createFormForType` is reused unchanged rather
than reimplemented client-side (which is why the server sends the resolved form
id instead of the client mirroring natsort ordering)
- [x] No security issues introduced — the map is a UI hint; `POST` re-authorizes
via the unchanged `OpCreate` + field-affordance gate. `_config` stays
principal-independent (`TestNavPermission_ConfigUnfiltered` passes), `_schema`
stays a pure metamodel projection.
- [x] No silent failures (errors surfaced, not swallowed)
- [x] No debug code left behind

**Gates:** `go test ./...` all green · `just lint` 0 issues · `just arch-lint`
OK · `just plimsoll` OK · `just coverage-check` PASS (77.3%) · frontend 1494
tests pass · e2e 235 pass (one server-startup flake under parallel load, green
in isolation).
