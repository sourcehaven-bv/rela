---
id: RR-AH4INS
type: review-response
title: Gated field without a grant must exclude the row
finding: SearchFieldScope drops grants whose fields a client ceiling narrows to nothing, so a gated field can have no grant. If an empty OR rendered as TRUE, a where on a hidden value would leak.
severity: critical
resolution: RowCondition with an empty Any holds for no row in the generic fold and in both SQL builders (pinned by storetest require_empty_any_holds_for_none). visibility.Rollups builds one condition per field the roll-up reads; a gated field without a grant yields an empty Any. Test with a client-ceiling principal.
status: addressed
---
