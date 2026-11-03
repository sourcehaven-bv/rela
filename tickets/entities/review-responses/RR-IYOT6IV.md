---
id: RR-IYOT6IV
type: review-response
title: Regex detection gives false positives and misses
finding: Text matching hit comments and literal content and missed anchored / tagged and nested-sequence headers.
severity: significant
resolution: Replaced the regex with the node walk. Test cases for anchored / tagged / commented headers and for header-like comments and literal content.
status: addressed
---
