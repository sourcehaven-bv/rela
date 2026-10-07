---
id: RR-QVBSEY
type: review-response
title: Rename can replace an empty directory created mid-run
finding: rename(2) replaces an empty target directory that appeared after resolvePaths.
severity: minor
resolution: Lstat the target again right before the rename and refuse if it exists. TestRun_TargetAppearsDuringImport.
status: addressed
---
