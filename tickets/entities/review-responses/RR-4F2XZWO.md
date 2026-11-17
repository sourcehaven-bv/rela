---
id: RR-4F2XZWO
type: review-response
title: 'Design: preview does not check the flows: section of the draft'
finding: 'Validate checked triggers only. flowcfg.Parse (flows: section and the 1 MB file limit) ran only at Prepare so a breaking draft passed preview.'
severity: significant
resolution: 'flowsProblems compares the parsed flows: section before and after and refuses any change (unsupported_change).'
status: addressed
---
