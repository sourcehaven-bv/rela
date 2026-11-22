---
id: RR-VJJO281
type: review-response
title: Redundant edge query in SubjectRuns
finding: SubjectRuns already found the run ids through the same _carries edge, so the carried query repeats it.
severity: nit
reason: One extra query per call; keeping views independent of its caller is worth more.
status: wont-fix
---
