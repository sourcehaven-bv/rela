---
id: RR-DQ19IN7
type: review-response
title: "Hidden empty dashboard cells shift the column headers"
finding: "On a phone, empty dashboard table cells had display: none. With the explicit row and cell roles, a screen reader then read every later value in that row under the wrong column header."
severity: significant
resolution: "Addressed: empty cells use the visually hidden pattern instead, so they stay in the accessibility tree and keep the cells after them under their headers. Checked at 360px: the empty cells are 1px high."
status: addressed
---
