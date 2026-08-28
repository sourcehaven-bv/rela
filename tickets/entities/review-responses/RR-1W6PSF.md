---
id: RR-1W6PSF
type: review-response
title: Todo.Complete invariant is not enforced — half-set completion renders a client-visible inconsistency
finding: |-
    The doc at calfeed.go:140 claims "This method exists so a caller cannot set half of it by accident." That claim is FALSE as written — all three fields are exported and independently settable. Both half-set states were constructed trivially and confirmed by two independent probes:

    - Todo{Status: TodoCompleted} renders STATUS:COMPLETED with NO COMPLETED property. Per RFC 4791 §7.8.9 the canonical pending-to-dos filter keys on COMPLETED being ABSENT, so this to-do reads as DONE in a STATUS-reading UI and PENDING in a filter-driven client — exactly the split-brain the method was added to prevent.
    - Todo{Completed: t} renders COMPLETED alongside STATUS:NEEDS-ACTION. Same disease, opposite direction.

    The mapping layer that will populate Todo from entity properties (TKT-UGYSC8) is the most likely place to set Status alone from a status column, so this is not hypothetical.

    Recommended fix (cheapest and most robust): normalise at the RenderTodo chokepoint rather than trusting the caller — if Status == TodoCompleted and Completed is zero, either omit STATUS:COMPLETED or fall back to DTSTAMP; if Completed is set, force STATUS:COMPLETED. This is consistent with how status() already defaults, and covers callers that bypass Complete().

    Alternative (rejected by the reviewer, and I agree): unexporting the trio behind Complete()/Reopen() accessors fights Go's struct-literal idiom for a leaf value type.
severity: significant
resolution: |-
    Fixed at the render chokepoint (commit 5a0cac4e). Added Todo.normalized(), applied first thing in RenderTodo and therefore inherited by TodoETag and the JSON path too, so all three renderings agree.

    Both directions handled asymmetrically on purpose: a COMPLETED timestamp promotes STATUS (and percent) because a timestamp is evidence the work finished, while STATUS:COMPLETED without a timestamp is DEMOTED to NEEDS-ACTION rather than inventing a completion time — fabricating one would be a lie about when the work finished.

    The false doc claim on Todo.Complete is corrected: it now states it is a convenience, not the guarantee, and points at normalized() for the actual enforcement.

    Pinned by TestRenderTodo_NormalizesCompletionTrio (both directions).
status: addressed
---
