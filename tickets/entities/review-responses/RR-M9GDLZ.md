---
id: RR-M9GDLZ
type: review-response
title: attachRender holder initialised in two places
finding: NewFromCollaborators and finishAssembly both set the holder.
severity: nit
reason: They are separate constructors and never both run on one Services value. The holder godoc now says so.
status: wont-fix
---
