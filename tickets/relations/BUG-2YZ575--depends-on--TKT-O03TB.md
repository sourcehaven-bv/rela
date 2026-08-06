---
from: BUG-2YZ575
relation: depends-on
to: TKT-O03TB
---

BUG-2YZ575 delivers the release-job half of the guard TKT-O03TB specified (build
+ assert in `.github/workflows/release.yml`). TKT-O03TB retains the broader
packaged-binary smoke test (spawn the binary, GET `/index.html` and a bundled JS
asset, wire into `just smoke`).
