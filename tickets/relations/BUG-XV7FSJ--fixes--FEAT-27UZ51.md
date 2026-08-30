---
from: BUG-XV7FSJ
relation: fixes
to: FEAT-27UZ51
---

The date/datetime property types are what this bug misformats: a `date`-typed
value is rewritten to a full RFC3339 timestamp on any write, so the declared
type no longer round-trips through the store.
