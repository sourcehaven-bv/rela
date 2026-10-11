---
id: RR-O6M7J6
type: review-response
title: Name case folding differs between backends
finding: kv uses strings.EqualFold, pg uses lower(name) under the database collation, which can disagree for some non-ASCII names.
severity: nit
reason: Each backend is internally consistent; the difference shows only for rare non-ASCII case pairs and only decides whether two names clash. Matching the database collation in Go is not worth the complexity.
status: wont-fix
---
