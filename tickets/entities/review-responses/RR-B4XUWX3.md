---
id: RR-B4XUWX3
type: review-response
title: Type alias passes validation but never matches
finding: GetEntityDef resolves aliases but the scope is matched against the stored canonical type.
severity: significant
resolution: Validation refuses an alias key and names the canonical type. Test case added.
status: addressed
---
