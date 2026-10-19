---
id: RR-PWS6DE
type: review-response
title: Face-anchored auto rule and the transient-failure path have no tests
finding: "internal/realtime/trigger_auto_test.go covers only family anchors. Nothing tests rule.anchor(ref) with FaceEach (trigger_rules.go anchor, the id@face seen keys) for an auto rule. Nothing tests a transient List or SetLifecycle error in systemWriter.transition: partial Moved, retry on the next round, the remaining channels moved, and give-up after TriggerRetryWindow with the entry marked seen.\tA regression in anchor() parsing for faced refs would close the wrong channel (or none) on anchor: face types, and no test fails. A change to the partial-failure contract described in the transition doc comment (\"a second call moves the rest\") goes unnoticed.\tAdd a table case with an anchor: face channel type on a faced entity, asserting only the entered face's channel closes. Add a test with a Store wrapper that fails SetLifecycle once on the second channel, asserting Moved over two rounds and the final states."
severity: minor
resolution: 'Added TestAuto_FaceAnchorClosesTheEnteredFace (anchor: face, only the entered face''s channel closes) and TestAuto_FailedMoveResumes (second SetLifecycle fails once; 1 moved, retry moves the other 2, all archived).'
status: addressed
---
