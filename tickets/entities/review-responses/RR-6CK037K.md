---
id: RR-6CK037K
type: review-response
title: Refresh broader than declared
finding: refresh_types covered every roll-up of a type and the SPA invalidated every list of the type.
severity: minor
resolution: ReadTypes covers only roll-ups a list reads; SPA invalidates only the declaring list; record writes emit no store event (checked).
status: addressed
---
