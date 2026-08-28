---
id: RR-OXE47R
type: review-response
title: AllowAll must skip the row gate but NOT redaction
finding: readQuery returns AllowAll when any effective global role grants read on the type (readquery.go:33-40). PLAN-VZXHRJ's diagram maps AllowAll to an unfiltered GraphQuery, which is correct for the ROW gate, but the plan never states that redaction must still run on that branch. A `visible:` policy can hide fields from a principal who has global read on the type, so conflating 'may read every row' with 'may see every field' on the AllowAll fast path silently un-redacts exactly the principals most likely to be reading in bulk. Same defect class as RR-1W1G6K but on the branch an implementer is most likely to treat as the trivial one.
severity: significant
resolution: Plan Approach now states redaction runs on every branch including AllowAll, with the reasoning ('may read every row' is not 'may see every field'). Added AC10 as a DEDICATED test rather than one shared with the Query branch, because AllowAll is where a 'nothing to gate, return rows straight through' shortcut is most tempting.
status: addressed
---

Raised by `/design-review` against PLAN-VZXHRJ, before implementation.

Worth pinning with a dedicated test rather than trusting the shared code path:
the `AllowAll` branch is the one where an optimization ("nothing to gate, return
the rows straight through") is most tempting and most wrong.
