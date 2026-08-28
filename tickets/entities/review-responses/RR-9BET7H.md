---
id: RR-9BET7H
type: review-response
title: Stale docs and comments become actively wrong — including the comment that encodes the root cause
finding: |-
    The plan flags 'docs' generically but names no sites. Concretely:

    1. internal/dataentryconfig/config.go:173-174 — 'VisibleWhen hides the field (and drops its value from the payload) when false'. Becomes false by default under the fix. This is the doc comment config authors actually read.
    2. docs/data-entry.md:604 — 'its values are dropped from the payload'. Same correction, plus the new key needs documenting alongside the condition-expression section (~:607-627), including the non-obvious point that `confirm` is not simply `yes` with a prompt because it can also undo the triggering change.
    3. DynamicForm.vue:200-204 — the watcher comment 'the edit analogue of create's submit-time hidden-branch pruning'. That comment IS the unsound symmetry argument identified as why4/the root cause. It must be DELETED, not edited — leaving it would re-seed the same reasoning for the next maintainer.

    Also: no migration should be added. internal/migration/ holds schema-shape migrations only; a behavior-default change has no precedent there and shouldn't get one — auto-adding clear_when_hidden: yes to existing fields would be a migration that PRESERVES the bug. Release note is the right vehicle.
severity: minor
resolution: |-
    All named sites updated:

    1. internal/dataentryconfig/config.go — FormField doc no longer claims VisibleWhen 'drops its value from the payload'; it now documents ClearWhenHidden and its three values. FormStep doc split create (values excluded) from edit (each field honors its own setting).
    2. docs/data-entry.md — 'Hidden branches are not saved' split into create vs edit; new #clear_when_hidden section with the value table, examples, and the chained-condition limitation.
    3. docs/data-entry/api-reference.md — _fields wire shape example gains a visible:false entry; the 'doubly invisible' paragraph rewritten to describe the tombstone, why absence was ambiguous, and why disclosing the field NAME is safe (the metamodel is already served in full). Per-row section notes it deliberately does NOT carry the tombstone.
    4. DynamicForm.vue:200-204 — the 'edit analogue of create's submit-time hidden-branch pruning' comment is DELETED, not edited. That comment was the unsound symmetry argument why4 identifies as the root cause; leaving it would re-seed the same reasoning.

    No migration added, per the finding's own reasoning.
status: addressed
---
