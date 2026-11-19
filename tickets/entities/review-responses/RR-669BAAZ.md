---
id: RR-669BAAZ
type: review-response
title: Shown nil contract is wrong
finding: 'The Nil: tag on ChromeToggle.Shown said nil is rejected, but nothing rejects it; it would panic.'
severity: minor
resolution: The comment now says permits must not be nil when Permission is set.
status: addressed
---
