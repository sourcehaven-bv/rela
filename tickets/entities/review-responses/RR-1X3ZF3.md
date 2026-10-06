---
id: RR-1X3ZF3
type: review-response
title: 'Small cleanups: docs, unused fields, temp dir, switcher negative'
finding: AGENTS.md omitted the history fixme file; FacedEntity carried unused fields; mkdtemp dir leaked if the writer threw; the switcher test lacked a negative.
severity: nit
resolution: Documented faces-history.spec.ts; dropped FacedEntity for EntityResponse; the project fixture removes the dir in finally; the switcher test asserts the draft body is gone.
status: addressed
---

AGENTS.md omitted the history fixme file; FacedEntity carried unused fields;
mkdtemp dir leaked if the writer threw; the switcher test lacked a negative.
