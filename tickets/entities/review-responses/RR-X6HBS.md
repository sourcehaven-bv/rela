---
id: RR-X6HBS
type: review-response
title: display_format forward-compat promise is shaky
finding: 'Plan claims display_property can be extended to recognize {...} placeholders without breaking anything. Two issues: literal ''{naam}'' as a property name silently changes meaning when format support lands; performance changes from O(1) lookup to per-call string parse without a documented caching strategy. The ''backward-compatible'' framing is hostage to fortune.'
severity: significant
resolution: Drop the forward-compat paragraph from the plan. The single-property design stands on its own. If display_format is ever needed, it'll be a separate decision with its own design (likely a parsed-template field cached on EntityDef, set during loader.go's parseRaw).
status: addressed
---
