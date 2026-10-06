---
id: RR-U2LNKK
type: review-response
title: Create menu reads the anchor at submit time, not at open time
finding: 'The create dialog lives in the app header and survives navigation. Open Create on topic A, navigate to topic B, submit: the row is linked to B. Capture the page scope when the menu row is chosen.'
severity: significant
resolution: SpaceCreateMenu resolves the link target when a menu row is chosen and keeps it beside the dialog. Pinned by a test that changes the page between open and save.
reason: ""
status: addressed
---
