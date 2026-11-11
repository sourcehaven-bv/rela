---
id: RR-3IDXMWQ
type: review-response
title: CI edit replaced the cross-compile job key
finding: The headless job was inserted by replacing the 'cross-compile:' line, so the Cross-Compile job's keys merged into the headless mapping as duplicates.
severity: critical
resolution: Restored the cross-compile key; a duplicate-key YAML load and actionlint now report no duplicates.
status: addressed
---
