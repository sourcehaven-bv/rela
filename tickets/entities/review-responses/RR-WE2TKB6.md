---
id: RR-WE2TKB6
type: review-response
title: Denied-write floods empty the Changes pages
finding: With the op filter in Go after LIMIT, an attacker's denied writes produce empty pages.
severity: minor
resolution: 'Plan: op filter in SQL (l.op IN allowlist), so pages hold only change records.'
status: addressed
---

With the op filter in Go after LIMIT, an attacker's denied writes produce empty
pages.
