---
id: RR-3B9X8R
type: review-response
title: setup-just installs the latest just release (no just-version)
finding: Pinning the action does not pin the just binary it installs. Without just-version, setup-crate downloads the latest casey/just release at run time. Pre-existing, outside this diff.
severity: nit
resolution: 'Deferred: set just-version in a follow-up if wanted.'
reason: Out of scope for TKT-H3Z4Z7, which pins only the action; behaviour is unchanged by this diff.
status: deferred
---
