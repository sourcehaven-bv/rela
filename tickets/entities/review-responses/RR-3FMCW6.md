---
id: RR-3FMCW6
type: review-response
title: Undeclared status restored with only a warning leaves an unleavable row
finding: A snapshot value the schema no longer declares was restored with a soft warning, leaving a row in a state no transition can leave.
severity: minor
resolution: EnforceRestore refuses a value no declared edge enters with ErrIllegalEntry (422). Pinned by TestTransition_RecreateEntity_UnenterableStatusIs422.
status: addressed
---
