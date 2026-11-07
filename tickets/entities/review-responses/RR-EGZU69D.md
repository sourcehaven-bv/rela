---
id: RR-EGZU69D
type: review-response
title: Singleton backstop index violation surfaces as an untyped error
finding: A unique violation on flow_runs_singleton_key comes back as a raw insert error instead of ErrSubjectBusy.
severity: minor
reason: Every writer takes the subject lock, so the index is only reached through a lock bug. A loud untyped error is the right signal for that; mapping it to ErrSubjectBusy would hide the bug and need a driver hook in sqlflowrun.
status: wont-fix
---
