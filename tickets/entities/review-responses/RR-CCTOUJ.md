---
id: RR-CCTOUJ
type: review-response
title: Family-delete fixme passes on any failure
finding: waitForToast accepted any toast, so a script crash or 500 would turn it green.
severity: critical
resolution: Chose refusal as the fixed behaviour and assert an error toast via FacesPage.expectErrorToast (data-testid toast-error) before checking both faces exist.
status: addressed
---

waitForToast accepted any toast, so a script crash or 500 would turn it green.
