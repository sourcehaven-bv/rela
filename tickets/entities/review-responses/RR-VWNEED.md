---
id: RR-VWNEED
type: review-response
title: Lint tripwire matched a literal call expression that a rename would break
finding: |-
    TestViewTraversalIsSourceGated asserted on the needle "h.readableViewIDs(ctx, next)" — an exact call expression. Renaming the local `next`, or extracting the assignment, breaks the tripwire with a failure that says "the gate is gone" when the gate is fine, and the cheapest path back to green is editing the needle rather than checking the gate.

    Given the test's own argument that "a prevention measure that disarms itself on refactor is worse than none", a brittle needle undercuts the point.
severity: minor
status: addressed
resolution: |-
    The needle is now the helper NAME, "readableViewIDs(", alongside "PermitsReadMany" and the newly-required "faceReadable(". Equally strong as a guard (the gate cannot be removed without removing the call) and far less sensitive to incidental edits. The rationale is recorded in a comment at the needle so the next person does not re-tighten it.
---

## Context

Found by code review of the BUG-9Z20WH fix.
