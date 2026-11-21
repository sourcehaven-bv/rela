---
id: RR-U48C8TG
type: review-response
title: "table-inline-edit e2e reads the hidden label"
finding: "expectTableCellText used toHaveText on the whole td, which reads textContent and so includes the display none label."
severity: critical
resolution: "Addressed: the helper asserts with useInnerText true; table-inline-edit.spec.ts passes."
status: addressed
---
