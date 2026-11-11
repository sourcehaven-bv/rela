---
id: RR-85FMUX8
type: review-response
title: Test helper uses a zero-value run store placeholder
finding: LockedFlowRuns wraps &sqlflowrun.Store{}, which panics if List or Get is called.
severity: nit
reason: Test-only helper; its doc says only Create and Update are reached, and the tests use only those.
status: wont-fix
---
