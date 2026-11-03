---
id: BUG-AR6NN9
type: bug
title: Provisioned principal loses its client ceiling
description: 'unmatched_principal: provision re-stamps with principal.Verified, which drops PrincipalType and Scopes, so a restricted client loses its ceiling on the provisioned request.'
priority: high
status: backlog
severity: high
---

## Description

`unmatched_principal: provision` re-stamps the request with
`principal.Verified(id, tool, org id, org slug, roles)` after it creates the
stub user (internal/dataentry/provision.go, `maybeProvision`). That constructor
keeps no `PrincipalType` and no `Scopes`, so the client ceiling of a restricted
client is lost on the provisioned request. The write and the read-back then run
with the acting user's full grants.

Principal resolution on the normal path (`access.Opener.Open`) rebuilds the
principal with `principal.VerifiedFrom` and the full claims for exactly this
reason.

Found in the design review of TKT-0NODIZ (a refactor, so the behaviour was kept
there).

## Fix

Rebuild the provisioned principal with `principal.VerifiedFrom` and every claim,
as `Open` does, and add a test with a scoped client principal that is
provisioned and then denied a write outside its ceiling.
