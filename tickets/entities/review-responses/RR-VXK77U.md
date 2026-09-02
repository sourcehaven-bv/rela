---
id: RR-VXK77U
type: review-response
title: app.go godoc brackets an unexported symbol from another file
finding: Go cannot link unexported members so [entityMutator] renders as literal brackets
severity: nit
resolution: Resolved - the doc paragraph containing [entityMutator] was rewritten for RR-NEE4FC and no longer brackets an unexported cross-file symbol. Verified no bracketed unexported refs remain.
status: addressed
---

`appEntityWriter`'s doc references `[entityMutator]`, an unexported symbol
declared in another file. Go cannot link an unexported member at all, so godoc
renders the brackets literally.

CLAUDE.md's `doclink` guidance is explicit that such references *"should simply
lose their brackets."*
