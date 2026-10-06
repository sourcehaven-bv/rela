---
id: RR-MJLN5N
type: review-response
title: Neighbor title reads loaded full bodies; loadRows allowlist reason stale
finding: Export, export-list and mentions loaded full rows only to derive titles, against the content-free collection rule, and mentions used loadRows for ungated ids although its allowlist reason said 'rows already gated'.
severity: significant
resolution: Added loadDefaultFaceHeaders (one header read via listIDHeaders, shared with loadStoredFamilies) and entityReader.defaultWorldHeaders; export, export-list and mentions use them. The loadRows reason now says every caller gates or redacts before serving. memstore headers now carry Redacted/Inaccessible like the other backends. World-neighbor include candidates keep full rows because they are served.
status: addressed
---
