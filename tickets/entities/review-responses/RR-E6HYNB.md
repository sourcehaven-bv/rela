---
id: RR-E6HYNB
type: review-response
title: Undocumented padding/margin standardization changes DocumentView/DocumentsPanel spacing
finding: 'Consolidating to shared rules silently changed spacing: DocumentView cell padding 10px 14px -> 8px 12px (tighter); DocumentsPanel table margin 12px 0 -> 16px 0 (looser). Defensible standardizations but undocumented.'
severity: minor
resolution: Added a docblock note in markdown-content.css recording the unified values (cell padding 8px 12px, table margin 16px 0) and the previous per-component values that changed (DocumentView padding 10px 14px, DocumentsPanel margin 12px 0).
status: addressed
---

**Finding:** Consolidating to shared rules silently changed spacing:
DocumentView cell padding 10px 14px → 8px 12px (tighter); DocumentsPanel table
margin 12px 0 → 16px 0 (looser). Both are defensible standardizations but
undocumented, so a future reader sees an unexplained density change. Add a
docblock note recording the unified values and the previous per-component ones.
