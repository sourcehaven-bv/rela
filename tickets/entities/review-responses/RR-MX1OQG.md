---
id: RR-MX1OQG
type: review-response
title: CLI waiting line prints role names; guide example shows the label
finding: 'internal/cli/channel_render.go:139 prints printable(w.Role), and the test pins "waiting for lead (USR-1)". GUIDE-realtime.md:1716 documents ", input_required, waiting for the incident lead (USR-1) (you)". Fix: print w.Label when it is set (it defaults to the role name), or correct the guide. Also " (you)" is appended after the whole "a (x) or b" list (channel_render.go:149-151), so it reads as if it belongs to the last role. Emit it as a separate ", waiting on you" clause.'
severity: minor
resolution: The CLI shows role labels (falling back to the name) and ends with "waiting on you"; tests and guide example updated
status: addressed
---
