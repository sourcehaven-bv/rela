---
id: RR-99JBU8
type: review-response
title: CLI show/render world semantics overlap PR 4
finding: show.go and render.go now use readAddress with svc.World, while PR 4 owns their world semantics.
severity: minor
reason: PR 5 must accept ID@face in every CLI address, which includes show and render. The overlap is a rebase concern, recorded in the PR report for PR 4.
status: wont-fix
---
