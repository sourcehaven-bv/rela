---
id: RR-HQDEL
type: review-response
title: workspace.State silently returned nopState on NewRootedFS error
finding: Returning nopState{} on NewRootedFS error would disable state persistence silently. Scheduler swallows Put errors, so misconfigured CacheDir would cause tasks to run missed-work on every restart forever.
severity: significant
resolution: 'State() now distinguishes two cases: (a) no cache dir configured (fs nil, paths nil, or CacheDir empty) returns nopState legitimately; (b) non-empty CacheDir that fails NewRootedFS panics with the offending path. The latter indicates programmer error and should fail loud. Tests that pass *project.Context with empty CacheDir take the legitimate-nopState path.'
status: addressed
---
