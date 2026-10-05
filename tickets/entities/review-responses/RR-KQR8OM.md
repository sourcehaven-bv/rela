---
id: RR-KQR8OM
type: review-response
title: Coverage line appears on faceless projects
finding: Every text report now prints a Coverage line, including on projects without faces.
severity: nit
reason: The coverage statement is true on a faceless project too, and a stable header is simpler for readers than one that depends on the schema. Documented in docs/cli-reference.md.
status: wont-fix
---
