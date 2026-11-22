---
id: RR-J5L6Y9B
type: review-response
title: Bad strings cause 503 instead of 400
finding: PostgreSQL rejects NUL in text and jsonb and out-of-range numbers, which came back as 503.
severity: minor
resolution: Change.Validate rejects control characters and invalid UTF-8 in the plan and string entitlements, non-finite floats, and numbers longer than 32 characters. Tested through PATCH.
status: addressed
---
