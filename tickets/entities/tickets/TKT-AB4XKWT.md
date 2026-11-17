---
id: TKT-AB4XKWT
type: ticket
title: Forward audit records to a SIEM through jobs.Queue
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

Follow-up of FEAT-0BH46JQ. A write-only audit implementation that forwards each
record to an external SIEM (syslog/HTTP). The send runs on `jobs.Queue`, never
inline on the write path (`.claude/rules/jobs.md`).
