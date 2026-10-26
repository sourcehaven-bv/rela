---
id: RR-MWNUFJ
type: review-response
title: Removed system:realtime guard lets a non-job mark retry as rela's own identity; writer does not fail closed on empty RunAs
finding: 'internal/realtime/outputs.go (deleted outputsFor shortcut ''mark.Writer.User == principal.UserRealtime -> no outputs'') plus internal/appbuild/realtime_outputs.go:95 writer. A 5a trigger close-only mark has Writer system:realtime and no Job field, so it decodes as Job=false. If it is still pending when this build starts with a config that now gives that transition outputs, resolveMark returns the outputs, identify() takes the retry branch and asRetry rebuilds VerifiedFrom("system:realtime"), so the graph write and its audit row carry rela''s own identity, which run_as validation refuses. 5a is not in any release tag, so only develop deployments can hold such marks. Separately, writer() falls back to the ctx principal (the leader''s system:realtime) whenever RunAs is empty, so a future path that reaches Write with a job mark and no run_as writes as system:realtime instead of failing. Fix: in run()/identify(), fail a non-job mark whose Writer.User is principal.IsRelaIdentity with forbidden (or keep the old no-outputs rule for !Job marks); carry Job on OutputRequest and make writer() refuse Job with RunAs == "" (OutputCodeForbidden, WroteNothing).'
severity: minor
resolution: writer() refuses an empty run_as, a rela identity and a retry as system:realtime; an old non-job mark by system:realtime runs no outputs but its close still runs; tested.
status: addressed
---
