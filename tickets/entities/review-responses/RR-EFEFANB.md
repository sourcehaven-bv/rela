---
id: RR-EFEFANB
type: review-response
title: Page verdicts are a snapshot
finding: 'Code review: An action script that changes a role-conferring edge while it iterates sees up to a page redacted as before the change.'
severity: minor
resolution: Stated in the redactingSeq godoc, with the note that the primed ctx never leaves the redactor so no write authorizes on it.
status: addressed
---
