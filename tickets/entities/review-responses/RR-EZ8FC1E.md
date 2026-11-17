---
id: RR-EZ8FC1E
type: review-response
title: 'Design: flow versions after a trigger-only rewrite are not checked'
finding: 'Apply may fall back to the encoder output and change node style in flows: which changes flow version hashes.'
severity: minor
resolution: 'flowsProblems compares the flows: section in the form the flow version hashes, so a save that would change a flow version is refused.'
status: addressed
---
