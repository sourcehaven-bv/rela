---
id: RR-SCXP2
type: review-response
title: Relation-card History selector used substring match against sequential ids
finding: 'historyButtonForCard used .entity-id:has-text("<id>"), a SUBSTRING match. With server-assigned sequential ids, "FEAT-3" also matches the card for "FEAT-30" — a strict-mode multi-match hard fail, or the absent-on-incoming assertion silently matching the wrong card. Latent flake.'
severity: critical
resolution: 'Match the .entity-id text with an anchored exact-id regex (^\s*<escaped id>\s*$) via a filter, so only the exact card matches. escapeRegExp helper added.'
status: addressed
---
