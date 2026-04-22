---
id: RR-BA10N
type: review-response
title: Config hot-reload race with open doc panels
finding: Editing data-entry.yaml to point a doc at a different script will flip rendered content on next SSE reload under the same doc name. Same-class issue as any config hot-reload; worth a one-liner in the guide.
severity: nit
resolution: Hot-reload caveat added to AC-DOC1 guide requirements.
status: addressed
---

From design-review on PLAN-78HJO. Minor caveat for the guide, no code change
needed.
