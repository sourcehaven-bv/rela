---
id: RR-33URZD
type: review-response
title: Contradictory expectation entries are accepted
finding: 'incident: [read, read:some] or t: [update] with t@face: [update] can never match.'
severity: minor
reason: Such a file fails on its first run with a diff that names the entry, so the error is visible. Load-time rejection is a convenience.
status: deferred
---
