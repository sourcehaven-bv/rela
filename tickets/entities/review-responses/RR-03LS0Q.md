---
id: RR-03LS0Q
type: review-response
title: list_comments exposed stored anchor descriptors
finding: comments.View embedded comments.Comment, so MCP served prefix, suffix, containing_sentence, heading_context and paragraph_index that HTTP never sends
severity: significant
resolution: 'Fixed: View is now a projection (NewView, AnchorView) with only kind, ref, quote and replacement, and CreatedAt in UTC to the second. newAnchorWire builds from the same Anchor.View, so HTTP and MCP share one projection. Pinned by TestNewView_ShowsOnlyTheReaderFacingAnchor.'
status: addressed
---
