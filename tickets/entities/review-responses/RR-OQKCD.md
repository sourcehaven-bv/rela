---
id: RR-OQKCD
type: review-response
title: Becomes-automation semantics on delete are not as documented
finding: 'Implementation checklist says ''becomes:<specific value> won''t fire'' — narrowly true, but the full picture is: deletion sets newValue="", so triggers with `becomes:""` will fire and triggers with bare `from:<oldvalue>` (no becomes) will fire because the diff matches. Today no automation in this repo uses those patterns, but a user could write one. Either suppress automation for delete events explicitly, OR update the documented contract to: ''deletion fires becomes:"" and bare-from triggers; only becomes:<non-empty> is guaranteed not to fire''.'
severity: significant
resolution: 'Documented the precise contract. Did NOT add code to suppress automation triggers on delete — that would change behavior for users who legitimately want a `becomes:""` trigger to fire when a property is cleared. The contract is now: deletion presents to the automation engine as ''set to empty string''. Triggers with `becomes:<non-empty value>` won''t fire; triggers with `becomes:""` and bare `from:<oldvalue>` (no becomes) will fire. This is the natural and minimum-surprise behavior. (Note: the implementation-checklist text was also imprecise on this and is corrected via the review-response trail rather than re-editing.)'
status: addressed
---
