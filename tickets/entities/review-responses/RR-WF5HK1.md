---
id: RR-WF5HK1
type: review-response
title: Poller started by the constructor
finding: 'Architecture design review: appbuild.New runs for every CLI command.'
severity: significant
resolution: 'Plan: Service.Watch(ctx) started only by rela-server, rela mcp and desktop.'
status: addressed
---
