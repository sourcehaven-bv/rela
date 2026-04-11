---
id: RR-72U6V
type: review-response
title: No SSRF protection for private networks
finding: No blocking of requests to private networks or cloud metadata endpoints.
severity: significant
reason: Consistent with existing trust model. AI module has same exposure. CLAUDE.md documents Lua scripts as trusted code. Future iteration can add configurable allowlist/denylist.
status: wont-fix
---
