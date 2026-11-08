---
id: RR-67RJROD
type: review-response
title: ListDue can starve later runs and has no cursor
finding: Runs that are due but cannot advance stay at the head and with a limit starve the rest.
severity: minor
resolution: ListDue takes a keyset cursor (due, id); harness case with stuck runs at the head.
status: addressed
---
