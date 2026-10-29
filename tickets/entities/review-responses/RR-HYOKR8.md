---
id: RR-HYOKR8
type: review-response
title: AC 10 omits session_output_denied
finding: A listed preset whose entry has outputs the caller's run cannot write is refused with 403 session_output_denied by the per-caller pre-check (internal/realtime/outputs.go:318-329). _actions.transitions has the same gap, so this is consistent, but list it in AC 10 as going through ui.handle and add a test.
severity: nit
resolution: AC 10 now lists session_output_denied as going through ui.handle, with an "output denied preset" test.
status: addressed
---
