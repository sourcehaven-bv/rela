---
id: RR-4TN7PD
type: review-response
title: The flattening guarantee had no test at its own seam
finding: 'The entire security property now rests on a single flattenToLine(...) wrapper inside interpolate (webhook_routes.go:781). Every test that covers it goes through the full HTTP router (newHookTestApp, ServeHTTP, listTickets). Delete the wrapper and the failures appear in end-to-end tests whose names talk about append_section — nothing points at interpolate, which is where the guarantee actually lives. A one-line guarantee with only end-to-end coverage is a guarantee waiting to be deleted by someone refactoring in good faith. Add a direct unit test on interpolate asserting a newline-bearing value is flattened regardless of destination, table-driven across the body/query/header/composite sources.'
severity: significant
resolution: 'Added TestWebhookPayloadInterpolate_FlattensEveryValue, a table-driven test calling interpolate directly, covering body newline, body carriage return, body NUL, query newline and header newline, plus two rows pinning the asymmetry itself: an operator newline in the template survives, and a value containing {{body.nl}} is emitted literally rather than re-scanned. A final assertion covers the composite case, where a map value is JSON-encoded so its newline arrives already escaped. Added TestWebhookPayloadInterpolate_ValueCannotForgeAHeading, which states the threat in the threat model''s own terms at the seam: no line of the result may begin with a heading marker, and the content must still be present rather than dropped. Both fail immediately if the flattenToLine call is removed from interpolate.'
status: addressed
---

The guard is one line. Its tests all drove the router, so the failure signal
named `append_section` rather than the seam that broke.

Both new tests call `interpolate` directly, so removing the guard fails a test
whose name is about interpolation. They also cover `query` and `header`, which
no previous test exercised for flattening at all.
