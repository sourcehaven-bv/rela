---
id: RR-792ON
type: review-response
title: Hardcoded aria-labelledby ID collides if two ConfirmModals mount simultaneously
finding: ConfirmModal uses a static id=confirm-modal-title with :aria-labelledby bound to the same string (and wrapped in a needless template literal). Teleport places the element at document.body, so two instances would create duplicate IDs. Generate a unique ID per instance via Math.random or Vue's useId().
severity: minor
resolution: ConfirmModal now generates a unique titleId per instance via `confirm-modal-title-${Math.random().toString(36).slice(2, 10)}`. The <h3> uses :id=titleId and the overlay's aria-labelledby binding is also :aria-labelledby=titleId (no more hardcoded literal). Vue's useId() was not used because it requires Vue 3.5+ and verifying the minimum version across all consumers felt like scope creep for a minor fix.
status: addressed
---
