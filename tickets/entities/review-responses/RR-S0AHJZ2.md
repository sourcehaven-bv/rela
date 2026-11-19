---
id: RR-S0AHJZ2
type: review-response
title: views reads each subject twice
finding: views already loads each subject through subjectOf; nameSubjects reads the headers again in one batch.
severity: minor
reason: The extra read is one batch per call and keeps one naming path for every view. It also names header-only runs once the finished-run fix (BUG-TVU9HRT) sets their subject.
status: wont-fix
---
