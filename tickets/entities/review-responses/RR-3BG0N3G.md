---
id: RR-3BG0N3G
type: review-response
title: Anchors and merge keys break the strict decode
finding: Re-marshalling the block lost aliases defined elsewhere and gave snippet line numbers.
severity: minor
resolution: Keys are checked on the node itself and node.Decode resolves aliases; errors carry file lines. Merge keys are refused explicitly; both tested.
status: addressed
---
