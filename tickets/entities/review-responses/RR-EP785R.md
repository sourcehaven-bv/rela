---
id: RR-EP785R
type: review-response
title: AC4 is a disjunction, so it defers a design decision into an acceptance criterion
finding: AC4 as written says the modal 'opens with an explicit empty state, or skips the modal — either is acceptable'. An implementer cannot write the test without first making the choice, so the choice gets made in code review instead of in planning. Compounded by the fail-closed relation fetch (relation_visibility.go:82-86), which makes 'no relations' and 'fetch failed' indistinguishable on the wire.
severity: minor
resolution: AC4 now picks the explicit empty state, with the fail-closed-fetch reasoning for why skipping the modal is wrong.
status: addressed
---

## Resolution

Pick the explicit empty state. Skipping the modal is wrong once the fail-closed
fetch is considered: a transient store error would silently become a
zero-relation duplicate with no confirmation step at all.
