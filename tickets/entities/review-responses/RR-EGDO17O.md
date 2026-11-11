---
id: RR-EGDO17O
type: review-response
title: A lagging run header lets a denied edit through
finding: 'flowengine patchHeader only logs a failed header patch (step.go:242-246), so the header can stay in a state that allows a property while the run is in a state that denies it. The plan''s reason for using headers (no run-store query by subject) is wrong: flowrun.Query.SubjectID exists on every backend.'
severity: significant
resolution: 'Addressed in plan: the ceiling reads the run store, which is authoritative and holds exactly the unfinished runs. flowrun.Query gains SubjectIDs so a page is one query; conformance-tested in flowruntest on kv, sqlite and postgres.'
status: addressed
---
