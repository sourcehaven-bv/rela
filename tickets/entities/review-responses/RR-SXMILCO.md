---
id: RR-SXMILCO
type: review-response
title: Enter on an invalid phrase commits the inline edit
finding: Enter on an invalid phrase commits the inline edit
severity: significant
resolution: RlInlineEdit skips an Enter the control already prevented; the phrase box prevents Enter only for a phrase it does not understand. Pinned by an InlinePropertyValue test that fails without the fix.
status: addressed
---
