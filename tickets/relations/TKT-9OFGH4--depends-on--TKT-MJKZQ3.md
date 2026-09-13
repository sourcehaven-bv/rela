---
from: TKT-9OFGH4
relation: depends-on
to: TKT-MJKZQ3
---

TKT-MJKZQ3 is the motivating consumer: its nested sections want status sorted
with completed last. Direction chosen deliberately — MJKZQ3 ships without this
fix (accepting alphabetical enum order), so it is not blocked. This edge records
that the sort work is driven by the nested section's needs, not that the section
waits on it.
