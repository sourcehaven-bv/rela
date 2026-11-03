---
id: RR-X9AYH3
type: review-response
title: Hidden edges count against the bound and leak via 422
finding: Edges the caller cannot read count against max_outgoing. A 422 can therefore reveal that a hidden edge exists.
severity: minor
resolution: Kept as is. The 422 message names only the caller's own entity and the bound.
reason: The bound is a data invariant, so it must count every edge, including edges the caller cannot read. Not counting them would let two users each add an edge and break the bound. The error names only the caller's own entity and the bound, not the hidden edge or its target.
status: wont-fix
---

Review finding R1-9.
