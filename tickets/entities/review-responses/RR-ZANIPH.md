---
id: RR-ZANIPH
type: review-response
title: 'Minor: fictional dry-run mock, modal-stubbing widget tests, createFormId never reset'
finding: (a) DynamicForm.embedded.test.ts mocks dryRunCreateEntity by echoing body.properties verbatim, but the real endpoint returns the affordance-PRUNED candidate — so the 'ignores host query string' assertion would pass even with the guard removed. (b) InlineCreate.widgets.test.ts stubs InlineCreateFormModal entirely, proving widget wiring but nothing about the modal. (c) createFormId is set on open but never cleared, so the modal component stays mounted-but-hidden after first use.
severity: minor
resolution: '(a) The query-guard tests now assert on the DRY-RUN REQUEST BODY, which carries the form''s live values before any affordance filtering — mutation-checked: removing the guard now fails the embedded case, which it did not before. (b) Added InlineCreateFormModal.test.ts (8 tests) covering the modal directly — the level that was missing, and where all three review criticals lived; mutation-checked, 5 fail against the pre-fix code. (c) Both widgets now clear createFormId on close via a shared closeCreateModal(), so the dialog unmounts instead of lingering hidden; the widget test asserts non-existence rather than show===false.'
status: addressed
---
