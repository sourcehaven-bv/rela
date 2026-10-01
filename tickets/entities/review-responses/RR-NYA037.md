---
id: RR-NYA037
type: review-response
title: Boolean cells render as a disabled checkbox
finding: Routing by server widget name sent booleans to CheckboxWidget; the dense-cell rule is boolean -> Yes/No text, and an input inside the link anchor is invalid HTML.
severity: significant
resolution: Dense routing maps boolean to text. Test asserts no input in the cell.
status: addressed
---
