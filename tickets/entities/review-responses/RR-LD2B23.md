---
id: RR-LD2B23
type: review-response
title: A pre-stamped QueryIdentity silently overrode the request principal
finding: appbuild's Match and Prefilters preferred an already-stamped QueryIdentity over the principal on the same ctx with no agreement check; the test even pinned bob's request selecting alice's rows. No production code stamps yet, so latent — but attachACLRequest refuses the analogous disagreement for ACL requests.
severity: minor
resolution: nextActionRequestScope errors (wrapped nextaction.ErrIdentityRequired) when a stamped identity and the request principal name different users; agreeing stamps and stamp-only contexts are honored. TestNextActionRequestScope pins all four cases.
status: addressed
---
