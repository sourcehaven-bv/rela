---
id: RR-H8S10M
type: review-response
title: 'verifiedPrincipal hardcodes ToolDataEntry, so AC #4 (audit Tool == mcp) fails and the obvious fix silently drops roles'
finding: 'router.go:576-578 returns principal.Verified(user, principal.ToolDataEntry, ...) with the tool hardcoded, so a remote MCP request is stamped Tool=data-entry. AC #4 requires Tool==mcp. Constructing a principal.Principal literal with Tool: ToolMCP instead would drop orgID/orgSlug/roles, since Verified is the only constructor that populates those unexported fields — the forbidden forge-proof-bypass path.'
severity: significant
status: addressed
resolution: >-
  Fixed in TKT-BDG8U9. verifiedPrincipal now takes the audit tool as a
  parameter (router.go), derived from the request path by toolForPath, so a
  request on /api/v1/_mcp is stamped Tool=mcp while still going through
  VerifiedFrom — the asserted org/roles/scopes are preserved rather than
  dropped. Pinned by TestRemoteMCP_AuditAttributionIsMCP, which asserts Roles()
  and OrgID() survive the swap; mutation-tested against the exact naive fix
  this finding warned about (a principal.Principal composite literal), which
  fails the test with empty roles and org.
---

## Finding

The plan left open: *"Which principal for `Tool`? Remote callers should
presumably still stamp `principal.ToolMCP`."* The code answers it, and the
answer is that it currently cannot.

`internal/dataentry/router.go:576-578`:

```go
return principal.Verified(
    user, principal.ToolDataEntry,   // ← hardcoded
    sanitizeUser(id.OrgID), sanitizeUser(id.OrgSlug), roles), true
```

A remote MCP request routed through the shared JWT gate is therefore stamped
`Tool: "data-entry"`, so **AC #4 fails as written** (it requires the audit
record to carry `Tool == mcp`).

**The obvious fix is the dangerous one.** Building a `principal.Principal{User:
..., Tool: principal.ToolMCP}` literal would compile and would set the tool —
but `principal.Verified` is the *only* constructor that populates the unexported
`orgID`/`orgSlug`/`roles`. A literal silently drops every asserted role, which
is precisely the forge-proof path the plan's own security item #5 forbids, and
would fail *open or closed* unpredictably depending on the policy.

## Resolution required

Either:
- add a `Verified`-preserving `WithTool` clone on `principal` (note
`clone_internal_test.go` already exists, so a clone seam may be present), or
- thread a `Tool` parameter through `verifiedPrincipal`.

Either way this modifies a security-critical shared function that the plan did
not budget for; add `internal/principal` and `internal/dataentry/router.go` to
"Files to modify".

**Check first** whether any `acl.yaml` policy or audit query keys on `Tool ==
"data-entry"`, since changing the stamp for the MCP route must not alter
data-entry behavior.
