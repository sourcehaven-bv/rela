---
id: RR-BZX4ZZM
type: review-response
title: triggers.New mutates shared notify hops
finding: 'triggers.New (resolveNotifyPaths in notify.go) writes n.hops into the NotifyAction held by the caller''s rules. SharedBase.triggers is parsed once and shared by every tenant''s engine, so assembling tenant B races tenant A''s planNotify. Fix: New works on a deep copy of the rules.'
severity: significant
resolution: triggers.New now works on a deep copy of the rules (cloneRules / unshareAction in engine.go). TestNew_SharedRules builds four engines from one []Rule concurrently and reported a data race under -race before the fix; TestNew_LeavesRulesUnchanged pins that the input is not modified.
status: addressed
---
