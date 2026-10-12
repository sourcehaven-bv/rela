---
id: RR-FFPNE9
type: review-response
title: PUT ownership via principal.Equal breaks on token refresh
finding: A refreshed JWT with changed claims would 404 every PUT while the stream stays open, silently killing subscriptions.
severity: significant
resolution: Ownership is decided on identity (User, Tool, RawUser, org); changed claims give 409 stream_principal_stale and the SPA reconnects (AC7).
status: addressed
---
