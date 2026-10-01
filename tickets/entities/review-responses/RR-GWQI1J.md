---
id: RR-GWQI1J
type: review-response
title: Silence under a client view was ambiguous
finding: A client view listing nothing could mean it reads the same as its role or reads nothing, so an operator could not tell a blocked exposure from a deduplicated one.
severity: significant
resolution: Added C0-same-as-role and C0-reads-none findings; any other client view lists all its exposures. Documented in docs/classification.md.
status: addressed
---
