---
from: BUG-OJNWVK
relation: depends-on
to: TKT-O03TB
---

BUG-OJNWVK is the second defect found by exercising the release path, and the
one a byte-level artifact guard could never catch (the release job never ran at
all). It is the concrete argument for TKT-O03TB's spawn-and-serve smoke test
running inside the `release` job.
