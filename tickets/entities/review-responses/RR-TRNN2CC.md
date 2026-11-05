---
id: RR-TRNN2CC
type: review-response
title: A fence line in a frontmatter block scalar hides a property conflict
finding: Fence tracking ran over the frontmatter, so a block scalar holding a fence line hid a later property conflict, which then failed as a YAML error.
severity: significant
resolution: Fence tracking is gone. TestConflictBlocks pins a fence in a block scalar before a frontmatter conflict.
status: addressed
---
