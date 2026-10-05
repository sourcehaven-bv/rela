---
id: RR-S1UY0F
type: review-response
title: View entry gate errors became a 422 with the raw error
finding: views_handler mapped every executeViewRef error to 422 view_execution_failed with err.Error() as detail. A read-gate fault leaked its raw text and had the wrong status.
severity: minor
resolution: viewEntry wraps resolver errors in gateFaultError and the view handler answers them with writeGateError.
status: addressed
---
