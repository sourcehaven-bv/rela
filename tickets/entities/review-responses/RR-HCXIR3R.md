---
id: RR-HCXIR3R
type: review-response
title: A token with several audiences crosses the boundary
finding: 'Security review: jwt.WithAudience accepts a token when any aud entry matches. A token minted with both the webhook and the tenant API audience would pass both verifiers, which undoes the audience separation.'
severity: significant
resolution: 'Addressed in the docs: the IdP must mint tenant API tokens with the tenant API audience only. The boot check already refuses equal audiences. rela cannot detect a multi-audience token issued for another purpose without breaking valid IdP tokens, so the rule sits with the issuer.'
status: addressed
---
