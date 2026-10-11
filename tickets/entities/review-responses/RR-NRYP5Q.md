---
id: RR-NRYP5Q
type: review-response
title: Detach rule and live-property set existed twice
finding: comments.Detached and livePropertyNames re-implemented resolveAnchor and liveAnchors; the HTTP side did not use them
severity: significant
resolution: 'Fixed: comments.LiveProperties is used by liveAnchors and by stdio; resolveAnchor decides detachment with comments.Detached and only computes the range itself. TestLiveProperties and TestDetached pin the shared rules; the existing HTTP tests pass unchanged.'
status: addressed
---
