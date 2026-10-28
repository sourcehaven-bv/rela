---
id: RR-VOFYPH
type: review-response
title: The plan applies the held-content rule to transient errors and keeps a dead item for no-identity callers
finding: Step 7 clears the summary on any error other than 404. frontend/CLAUDE.md says to drop held content only when shouldDropHeldContent(err) is true and to keep it on every other failure, so a network error should not blank the badge. AC 16 and AC 17 should say the same. Also, a caller without an identity gets 403 no_principal (authz.go:87,93-97). Under the plan the Inbox item then stays forever, without a badge, and errors when clicked. Hide the item on no_principal as well as on 404.
severity: minor
resolution: The summary drops held content only when shouldDropHeldContent(err) is true and keeps it on other errors, the item hides on 404 and 403 no_principal, and AC 20 and AC 21 say so.
status: addressed
---
