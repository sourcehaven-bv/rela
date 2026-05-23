---
id: RR-12KX
type: review-response
title: Removed disabled attribute changes AT semantics during round-trip
finding: Removing 'disabled' makes checkboxes a tab stop, focusable, keyboard-operable via Space. But the underlying semantics are 'remote-controlled' — keyboard-toggle native behavior is intercepted by e.preventDefault(). AT users get a focusable input whose state doesn't update visibly until the server responds, with no aria-busy during the round-trip.
severity: minor
reason: Accessibility hardening (aria-busy during round-trip, aria-checked mirroring, focus-visible affordance during latency) is a separate workstream that should cover all interactive markdown content uniformly, not just the checkbox toggle. The pre-fix state had AT users seeing 'checkbox, disabled' — also wrong but with worse UX. The fix already improves the AT story by making checkboxes operable; full ARIA hardening is a follow-up that would be premature here without a broader audit.
status: deferred
---
