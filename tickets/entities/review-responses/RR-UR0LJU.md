---
id: RR-UR0LJU
type: review-response
title: 'A9 wildcard-sprawl would false-positive on the documented `everyone: read: ["*"]` example'
finding: 'The canonical worked example in docs/acl-overview.md uses `everyone: read: ["*"]` — a legitimate, recommended pattern (everyone can read everything). If A9 flags any wildcard grant it fires on the docs'' own example, training operators to ignore the audit on first run. Fix the gating: A9 must only flag WRITE/permission wildcards (create/update/delete: ["*"] or a delegate permission) on NON-everyone roles, and never read:["*"]. Read-everything is an intentional visibility choice, not least-privilege sprawl. Add an explicit negative test: `everyone: read:["*"]` → zero A9 findings.'
severity: significant
resolution: 'A9 gating tightened: only write/permission wildcards flagged, never read:[''*'']. Mandatory negative test added (everyone: read:[''*''] → zero), plus the full docs worked-example policy as a golden zero/expected-findings fixture.'
status: addressed
---
