---
id: RR-2KZCA3
type: review-response
title: No test covers a non-default request world
finding: Every test varies the configured default world and requests the bare path, so resolving in the default world instead of the request world would still pass.
severity: significant
resolution: Added TestMentions_ResolveInTheWorldTheRequestNames (?world=drafts serves the draft title on both routes) and TestMentions_DeniedWorldServesNoMention.
status: addressed
---
