---
id: BUG-OQQE8D
type: bug
title: GraphCount total under a world with Any differs between naive and pgstore
description: GraphCount's total under a world with Any branches applies the branch face sets in graphquerynaive but not in pgstore.
priority: low
effort: s
status: backlog
---

## Description

Under a non-default world with `Any` branches, `GraphCount`'s total differs by
backend. graphquerynaive's `Count` sets the total to `len(collectByType(...))`,
which applies the branch face sets (`collectBranchPrimes`). pgstore's
`buildGraphTotalSQL` counts every world prime of the type and ignores `Any`.
Under the default world naive ignores `Any` for the total too, so naive is also
inconsistent with itself.

Decide what the denominator means under ACL branches, pin it in storetest, and
make both backends agree.

Found in the BUG-2SKLD3 code review.
