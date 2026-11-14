---
id: RR-ZA4MEVR
type: review-response
title: Overloaded endpoint and tool for batches
finding: Reusing POST .../messages and post_message for batches mixes two request and response shapes.
severity: nit
resolution: 'Plan: separate POST .../_batch, MCP post_messages and CLI channel post-batch.'
status: addressed
---
