---
id: RR-NSKU
type: review-response
title: hashLogoBytes collision comment is misleading
finding: theme_logo.go::hashLogoBytes comment claims 'a no-op collision (same bytes) would be harmless' — but a *different* image hashing to the same prefix would cause stale-cached delivery, not a no-op. Rewrite to call out that scenario and note the probability is effectively zero on a single-logo workspace.
severity: nit
resolution: Rewrote hashLogoBytes godoc to call out the actual collision consequence (stale cached image until expiry) instead of the misleading 'no-op' wording, and to note the probability is effectively zero on a single-logo workspace.
status: addressed
---
