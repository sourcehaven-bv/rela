---
id: RR-B6JXL9Q
type: review-response
title: Find can focus a hidden search box
finding: querySelector took the first match in the DOM, which can be a list hidden by v-show or in an inactive panel.
severity: minor
resolution: Find takes the first match with a layout box (getClientRects). Covered by the new hidden-box test.
status: addressed
---
