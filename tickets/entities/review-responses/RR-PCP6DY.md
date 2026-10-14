---
id: RR-PCP6DY
type: review-response
title: Flows do get require; yield during module load undefined
finding: rela flow builds its runtime via script.NewWriterRuntime with real Files, so require works there, contrary to the plan's scope text. A module whose top level yields inside the Go-side ls.Call has undefined behaviour in gopher-lua.
severity: minor
resolution: 'A module''s top level runs in its own coroutine thread; a yield there raises ''yielded while loading'' instead of corrupting the caller (it previously caused a nil dereference on resume). Plan scope corrected: flows do get require. Pinned by TestRequire_YieldDuringModuleLoadIsAnError. Separately found a pre-existing gopher-lua crash on tail-call ''return coroutine.yield(...)'', unrelated to modules.'
status: addressed
---
