---
id: RR-RBWQ2IR
type: review-response
title: No DOMPurify backstop and a second link policy next to utils/markdown.ts
finding: No DOMPurify backstop and a second link policy next to utils/markdown.ts
severity: minor
reason: The library may not depend on the app sanitizer; custom renderers escape every token and probes held. Revisit if marked is upgraded.
status: deferred
---
