---
id: RR-YB3V9N6
type: review-response
title: Test assertion never reached the no-evaluator branch
finding: s.capture on the reloaded scheduler failed at taskNamed, so the cards == nil branch was uncovered.
severity: minor
resolution: The test stores a config with the capture task directly and asserts the no-evaluator error.
status: addressed
---
