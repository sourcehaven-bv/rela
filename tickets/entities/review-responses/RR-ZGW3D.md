---
id: RR-ZGW3D
type: review-response
title: waitForSpinnerToDisappear has a flaky 100ms pre-check
finding: 'If spinner appears AFTER probe, pre-check skips and you don''t wait. Just `await expect(spinner).not.toBeVisible({timeout: 3000})` — passes if already hidden.'
severity: nit
reason: Nit. The 100ms probe works for the known spinner pattern; changing it risks introducing different flake. Defer unless the described race actually triggers.
status: deferred
---
