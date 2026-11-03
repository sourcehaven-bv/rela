---
id: RR-LU6G3J
type: review-response
title: Runner reads the config snapshot several times per mark
finding: 'runOne calls outputsFor, which does a cfg.Load, for readsTitle (internal/realtime/outputs_runner.go:136) and run calls it again (outputs.go:582). The plan adds more lookups of session.RunAs() and the suspended flag. A reload between them can pair one generation''s outputs with another''s run_as, which goes against CLAUDE.md ''Capture state once per operation''. Plan change: load the snapshot once in runOne, resolve the transition or timer, outputs, run_as and suspension from it into the markRun, and pass that through.'
severity: minor
resolution: runOne loads the snapshot once and resolves transition/timer, outputs, readsTitle, run_as and on_final into markRun; run and outputsFor read only markRun (Approach 11, AC 17).
status: addressed
---
