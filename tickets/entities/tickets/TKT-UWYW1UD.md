---
id: TKT-UWYW1UD
type: ticket
title: 'Tenant review follow-ups: validate org_id on the request path, fix upgrade guide and plan scope'
kind: chore
priority: low
effort: s
started: "2026-11-10"
completed: "2026-11-10"
status: done
---

## Description

Follow-ups from the ISMS review of #2171 (TKT-HF5KE12):

- #2174: the request path (Resolve/Lookup via SchemaFor) only refuses an empty org_id; the threat model says checkOrgID runs there.
- #2172: the postgres upgrade guide uses `tenant show` before the control table exists, which fails.
- #2173: PLAN-W11X48M scope lists CLI revalidation as out of scope, but #2171 built it.
