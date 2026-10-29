---
id: RR-C399SN
type: review-response
title: Unlabelled PoC types render as lowercase names
finding: ChannelTypeSchema documents that a label falls back to the name (types/channels.ts ~500). In the motivating realtime.yaml only go, approve and revise have labels, so the buttons read 'allow', 'deny', 'accept', 'rework', 'retry', 'abandon', 'answer', and the confirm title and confirm label read 'abandon'. The plan forbids derived labels, which is fine. The docs paragraph should tell authors that a decision type needs a `label:`, and the PoC config should get them (outside this repo).
severity: minor
resolution: 'Docs tell authors to give decision types a label: (and in:), and the plan notes the PoC config needs labels outside this repo.'
status: addressed
---
