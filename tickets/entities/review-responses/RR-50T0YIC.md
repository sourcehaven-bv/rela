---
id: RR-50T0YIC
type: review-response
title: audit:read exposes every entity's existence; audit the read
finding: The global view is a row-existence bypass comparable to history:read-redacted.
severity: minor
resolution: 'Plan: guide states the disclosure; each /_audit request appends an audit-read record (principal, cursor, count; no content).'
status: addressed
---

The global view is a row-existence bypass comparable to history:read-redacted.
