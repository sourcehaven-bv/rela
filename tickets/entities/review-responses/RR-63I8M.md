---
id: RR-63I8M
type: review-response
title: '''blur commits immediately'' undefined for non-input widgets (markdown editor, RelationPicker, SlimSelect)'
finding: |-
    Plan says blur should commitImmediately() but doesn't define blur for non-<input> widgets. EasyMDE has its own focus model; RelationPicker, RelationCards, SlimSelect dropdowns each manage focus differently. A click on a card's checkbox isn't blur — it's a synthetic event.

    Fix: bind commitImmediately() to the form-root focusout event with a microtask delay (so focus moving between form widgets doesn't fire commit, but focus leaving the form does). Test: focus moves between three property fields rapidly — only one commit at the end.
severity: minor
resolution: 'commitImmediately() bound to form-root focusout with microtask delay (queueMicrotask) — focus moving between form widgets doesn''t fire commit, but focus leaving the form does. Test: rapid focus-cycle between three fields produces a single commit at the end.'
status: addressed
---
