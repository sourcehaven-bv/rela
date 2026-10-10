---
id: RR-HMH5G2
type: review-response
title: Raw path in attachment warnings
finding: handlers_attachment.go logged r.URL.Path on write and delete failure (security review).
severity: minor
resolution: Both use shapedPath.
status: addressed
---
