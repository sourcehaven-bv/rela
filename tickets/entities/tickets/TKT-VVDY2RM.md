---
id: TKT-VVDY2RM
type: ticket
title: 'Flows: mail job kind'
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

A `job:` step with `kind: mail` (split off TKT-PY2G11W by its design review, RR
on the mail recipient).

Scheduled mail renders a template under the recipient's principal, which the
scheduler installs per recipient. A flow job runs as the flow's `run_as`. If the
job rendered the template as `run_as`, the recipient would receive what `run_as`
may read, which can be more than the recipient may read.

## Scope

- Resolve the recipient (the subject or a `to:` relation) to its principal
the way scheduled mail does, and render as that principal.
- A recipient without a principal is a permanent failure.
- Classify SMTP replies: 5xx permanent, 4xx and network errors before DATA
retryable, an error after DATA an unknown outcome.
