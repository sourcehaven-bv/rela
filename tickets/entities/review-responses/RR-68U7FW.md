---
id: RR-68U7FW
type: review-response
title: SessionStep gets both OutputsMark and Mark
finding: 'Decision 5 adds `SessionStep.OutputsMark *OutputMark` and a `Mark func(move SessionMove) *OutputMark` callback. Only the callback can decide per move. Plan change: keep only Mark, and SessionStep grows by one field.'
severity: nit
resolution: Only the Mark callback is kept, so SessionStep grows by one field.
status: addressed
---
