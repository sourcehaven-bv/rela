---
id: RR-DSILLV
type: review-response
title: Ragged godoc reflow
finding: 'The edited godoc paragraphs were not rewrapped: configure.go:93, projectsetup/validate.go:284, cli/validate.go:578 and realtime_wiring.go:49 each have one line far longer than its neighbours. Rewrap to the paragraph width.'
severity: nit
resolution: ADDRESSED Rewrapped the four godoc paragraphs in configure.go, projectsetup/validate.go, cli/validate.go and realtime_wiring.go.
status: addressed
---
