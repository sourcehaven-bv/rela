---
id: RR-4P8I0
type: review-response
title: 'Edit keyboard shortcut: decide explicitly'
finding: EntityDetail.vue:400-402 renders the Edit button with a <kbd>E</kbd> hint and wires the 'e' key to editEntity (lines 43-46). The plan proposes a button without a shortcut. Either drop intentionally and say so, or wire 'e' (cribbing the input-focused/modal-open guards from EntityDetail.vue:37-69, not optional). Pre-empt the 'while you're here' reviewer.
severity: nit
reason: Out of scope per the original ticket. The plan's Alternatives section explicitly defers it. Track separately if we want consistency with EntityDetail's 'e' shortcut.
status: deferred
---
