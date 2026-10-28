---
id: RR-TVB3SH
type: review-response
title: Empty confirm labels render an empty button
finding: useConfirm now defaults confirmLabel/cancelLabel to an empty string and ConfirmModal falls back with ??, which keeps the empty string.
severity: nit
resolution: ConfirmModal falls back with ||, so an empty label shows common.confirm / common.cancel.
status: addressed
---
