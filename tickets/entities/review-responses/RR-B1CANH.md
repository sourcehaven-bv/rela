---
id: RR-B1CANH
type: review-response
title: Copy paths lack unit tests for partial and cross-entity copies
finding: No test pinned a partial copy, a mapped mint, or a cross-entity copy of a file property.
severity: minor
resolution: Added TestFaced_PartialCopyKeepsTargetUpload, TestFaced_CopyMappingIntoFilePropertyRefused and TestFaced_CrossEntityCopyKeepsTargetFileValue.
status: addressed
---
