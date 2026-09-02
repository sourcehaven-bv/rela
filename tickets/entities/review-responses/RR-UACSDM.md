---
id: RR-UACSDM
type: review-response
title: kong.go exception rationale claimed 'nothing reads these fields' — runKong reads all four globals 78 lines later
finding: 'The comment written to justify cli.CLI''s max-fields=46 structural exception asserted ''nothing reads these fields except kong.New and the build test''. False: runKong (kong.go:145-148) reads cli.Verbose, cli.Quiet, cli.Output and cli.Project directly — precisely the 4 global flags the same comment enumerates, so the sentence is self-refuting against its own arithmetic. This matters because the comment IS the deliverable of that hunk: it replaced a TODO ratchet with a documented justification, and an exception resting on a demonstrably wrong premise is worse than the honest TODO, since the next reader accepts it and never re-examines.'
severity: significant
resolution: 'Rewrote to the narrower claim that is actually true: the 42 subcommand fields are kong-dispatched and read by nothing else, and the 4 globals are read once, in runKong. Verified against kong.go:145-148 before editing.'
status: addressed
---

Significant finding from the TKT-NS3XPE code review (cranky-code-reviewer, PR
#1469).

Reviewer's verdict on the substance was a clean pass, with the deletion verified
rather than assumed: WriteRelations had zero production callers across all three
build tags (default/memorybackend/postgres), cmd/, e2e/, non-Go dispatch
surfaces, and there is no MethodByName anywhere in the repo so no reflective
dispatch could break. Coverage specifically checked per-symbol rather than by
floor: newBorderlessTable, writeSeparator and writeFooterSummary are each at
100%, and all six moved schema methods are at 100% in their new home; 6 of the 8
deleted tests moved 1:1 with byte-identical assertions. JSON output diffed
byte-identical (same map keys, same SetIndent). plimsoll runs silent with the
directive genuinely deleted, not leaning on a leftover pin.
