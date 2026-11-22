---
id: RR-R2BW4OE
type: review-response
title: "Direct showToast calls bypass the suppression"
finding: "HistoryView and RelationHistoryView call showToast with the error type directly, so a check in error() and warning() does not cover them."
severity: significant
resolution: "Addressed: the check moved into showToast for error and warning toasts; the test covers the direct call."
status: addressed
---
