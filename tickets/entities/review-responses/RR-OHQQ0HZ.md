---
id: RR-OHQQ0HZ
type: review-response
title: Cancel races a concurrent step
finding: Cancel read the run, then deleted it without a compare-and-set, so a step that moved the run in between was lost and the header could record the wrong state.
severity: significant
resolution: Cancel now loops Get then Delete at the read rev (8 tries) and patches the header only after its own delete wins. A run that ended meanwhile answers ErrRunNotFound.
status: addressed
---
