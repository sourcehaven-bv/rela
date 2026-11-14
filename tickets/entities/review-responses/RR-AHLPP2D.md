---
id: RR-AHLPP2D
type: review-response
title: No test covers the hidden-edge cost of the gated page
finding: paddedACLWorld pads only visible edges so the scale-free test never exercised hidden rows or a gate fault.
severity: minor
resolution: Added TestScriptReader_ListRelationsPageHiddenEdgesCostChunks and TestScriptReader_ListRelationsPageReturnsGateFaults in internal/visibility.
status: addressed
---
