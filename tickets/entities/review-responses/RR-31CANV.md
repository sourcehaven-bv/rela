---
id: RR-31CANV
type: review-response
title: mcp arch-lint whitelist lacks acl and attachment
finding: 'Planned imports of acl (WriteRequest/Decision) and attachment (errors, Info, Result) from internal/mcp are not allowed by .go-arch-lint.yml. Fix: add both with a justification comment.'
severity: minor
resolution: Added acl and attachment to mcp.mayDependOn in .go-arch-lint.yml; just arch-lint is clean.
status: addressed
---
