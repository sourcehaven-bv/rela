---
id: RR-BACHJMD
type: review-response
title: Webhook signature allows replay and bypasses the HTTP seam
finding: Signing only the body lets a captured request be replayed later; a default http.Client bypasses the services' HTTP seam and follows redirects.
severity: minor
resolution: 'Plan: sign timestamp.body with X-Rela-Timestamp; use the services'' HTTP seam when set; no redirects.'
status: addressed
---
