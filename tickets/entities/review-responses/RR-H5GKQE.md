---
id: RR-H5GKQE
type: review-response
title: Dialog buttons are not PendingButton
finding: frontend/CLAUDE.md requires PendingButton with a pendingLabel for buttons that trigger a request.
severity: minor
resolution: The dialog buttons now use the RlButton pending props (loading and pending-label), the library form of PendingButton.
status: addressed
---
