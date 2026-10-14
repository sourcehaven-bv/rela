---
id: RR-YBE2ZI
type: review-response
title: Provision re-stamp drops the client ceiling
finding: maybeProvision rebuilds the principal with principal.Verified, which keeps no PrincipalType or Scopes, so a restricted client loses its ceiling. Moving it next to resolvePrincipal, which uses VerifiedFrom, would hide the divergence.
severity: significant
resolution: Filed BUG-AR6NN9 with the fix and test plan. Opener.Restamp names the bug in a comment. Behaviour is kept because this ticket promises no behaviour change; TestOpen_ResolutionKeepsClaims pins the correct path.
status: addressed
---
