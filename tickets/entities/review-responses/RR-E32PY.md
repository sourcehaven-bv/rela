---
id: RR-E32PY
type: review-response
title: formatDate not exported, invites reinvention
finding: formatDate is private, so the next view that displays a date will write inline new Date(x).toLocaleDateString() and reintroduce ambiguous numeric output. Export formatDate (and DATE_FORMAT_OPTIONS) so it's the obvious thing to grab.
severity: significant
resolution: Both formatDate and DATE_FORMAT_OPTIONS are now exported from frontend/src/utils/format.ts. Future callers needing date display can import formatDate directly instead of inlining toLocaleDateString.
status: addressed
---
