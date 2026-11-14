---
id: RR-H3YG95N
type: review-response
title: Two loaders for the config-check policy
finding: flows and realtime each read acl.yaml under an injected ACL in their own way.
severity: minor
resolution: Extracted loadConfigPolicy in realtime.go and used it in both.
status: addressed
---
