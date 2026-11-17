---
id: RR-ZWGTUK3
type: review-response
title: Empty draft creates a stray flows.yaml
finding: 'Removing the last automation in a project without flows.yaml saved a file with triggers: {}.'
severity: minor
resolution: 'Prepare skips a flows draft that holds only empty mappings while the file is absent. Test: TestSave_EmptyDraftLeavesFlowsAbsent.'
status: addressed
---
