---
id: RR-CY6WIFE
type: review-response
title: PageQuery.Related is trusted, not checked
finding: visibility.PageQuery.Related is ANDed onto the read scope without the reader checking that each entry passed the caller's traversal gate. A future caller that builds an ungated entry would let the row set depend on edges the caller cannot see.
severity: significant
reason: The field only narrows the gated read scope; it can never widen it. The trust model matches data-entry's listPlan, which passes gated Related entries the same way. The only producer is appbuild.QueryConditions.Lower, which gates every entry with the bundle's traversal gate. The security review found no issue. Making the reader accept only gated entries needs a typed gated-relation value across appbuild and visibility; tracked in TKT-911BIMT.
status: deferred
---
