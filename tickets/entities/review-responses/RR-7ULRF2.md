---
id: RR-7ULRF2
type: review-response
title: Default label duplicates the type for screen readers
finding: 'internal/dataentryconfig/config.go:1543-1545 and frontend/src/types/config.ts:605 label a busy field with the raw channel type key. show_label defaults to true, so a default card renders "run:" before the indicator, and a screen reader hears "run: Agent run: Planning, for 3 hours" (KanbanView.cardBusy.test.ts:138-145 pins the "run:" label). The raw key is less helpful than type_label, which the server already sends. Fix: make LabelShown() default to false for busy fields (the indicator names its own type), or label the field with the channel type''s label. Update the docs and tests to match.'
severity: minor
resolution: A busy field now hides its label unless show_label is true, in Go (LabelShown) and in cardFieldLabelShown; tests in validate_test, CardFieldList and KanbanView.cardBusy; guide and e2e board updated.
status: addressed
---
