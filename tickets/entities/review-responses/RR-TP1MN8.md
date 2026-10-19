---
id: RR-TP1MN8
type: review-response
title: CLI watch exits 0 on both change and timeout
finding: Scripts in table mode cannot tell the two apart without parsing text. Consider a distinct exit code for timed_out, or document the -o json check.
severity: nit
resolution: Timeout exits 2; a change exits 0; an error exits 1.
status: addressed
---
