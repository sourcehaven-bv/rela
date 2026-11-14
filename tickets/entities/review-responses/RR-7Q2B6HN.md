---
id: RR-7Q2B6HN
type: review-response
title: Three functions named for compiling transitions return different types
finding: appbuild.CompileTransitions returned a TransitionWiring while flowcfg.CompileTransitions returns Transitions; the CompileHeadlessFlows godoc claimed every compile site uses it.
severity: minor
resolution: appbuild.CompileTransitions is renamed BuildTransitionWiring; the CompileHeadlessFlows godoc names the data-entry resolver as the one site with its own binder.
status: addressed
---
