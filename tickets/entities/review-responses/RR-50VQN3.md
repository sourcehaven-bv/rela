---
id: RR-50VQN3
type: review-response
title: 'keep: has no test for a non-file property'
finding: The only behavioural test kept a file property and did not check the source face's bytes after the sweep.
severity: minor
resolution: Added TestCopyKeep (existing and new target) and a source-bytes check in TestFaced_CopyKeepLeavesTargetUpload.
status: addressed
---
