---
id: RR-3EPC0W
type: review-response
title: Face plumbing into gantt nodes is untested
finding: 'The edge test builds ganttNode{face} by hand; dropping Face: red.Face at the load sites would go unnoticed.'
severity: minor
resolution: TestGanttNodes_KeepTheirFace feeds addGanttNodes a faced row and asserts node.face.
status: addressed
---
