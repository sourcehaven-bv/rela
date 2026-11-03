---
id: RR-3BWQ1L
type: review-response
title: Two copies of the session-notice check, and a duplicated Drives check
finding: 'render.go:93 MessageView.IsSessionNotice and session.go:244 isNotice repeat the same type-plus-marker rule. service.go:336 checks Session()/Drives before calling sessionStep, which checks both again. Fix: define the rule once, e.g. realtime.IsNoticeOf(type, triggeredBy), used by both. Drop the outer check in Post and compute transitionRoles lazily, or pass the poster builder.'
severity: nit
resolution: One isNotice helper serves the service, the renderer and MessageView.IsSessionNotice; the duplicate Drives check is gone.
status: addressed
---
