---
id: RR-HW0EPK
type: review-response
title: An acl.yaml reload that removes one run_as grant degrades realtime as a whole
finding: 'Parse failure keeps the previous config with Degraded set for every channel type (appbuild/realtime.go:197). An operator who edits acl.yaml for an unrelated reason and drops system:breach-bot''s update grant freezes the timers, the runner and the triggers of every realtime type, not only the session type that uses that run_as. The runtime ACL is authoritative anyway. Plan change: either accept this and document it under "Reloading" with the problem key, or treat a grant that a reload removes as a per-type problem (the timer outputs of that type fail `forbidden` and are visible) while the initial load stays strict.'
severity: minor
resolution: 'Deferred to 5b: how an acl.yaml reload that drops a run_as grant is handled is listed under Deferred to 5b.'
status: addressed
---
