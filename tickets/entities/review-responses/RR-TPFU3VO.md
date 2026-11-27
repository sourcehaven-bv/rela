---
id: RR-TPFU3VO
type: review-response
title: A new lead does not start an outputs round at once
finding: lead() runs everything once at start but starts the outputs runner only through marksWake, so marks left by a previous lead wait for the first minute tick.
severity: minor
reason: 'Pre-existing and outside this bug: it costs at most one minute after a leader change and does not lose a wake within a lead. Worth its own ticket if leader changes become frequent.'
status: deferred
---
