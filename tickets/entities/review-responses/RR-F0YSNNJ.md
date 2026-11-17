---
id: RR-F0YSNNJ
type: review-response
title: who-can credits routes for subject types the run gate drops
finding: '[security] flowRunReadRoutes credited every non-ancestor route on a readable subject, while flowRunReadQuery drops a subject type whose read cannot fold into an endpoint (face-split or several role relations with one when:). AccessRoutes checks PermitsRead on the run first, so no wrong route reached the output, but the two were not tied together.'
severity: significant
resolution: runSubjectHop builds the run gate's endpoint per subject type; flowRunReadQuery and flowRunReadRoutes both use it, so who-can credits a type only when the gate keeps its branch. Docs list the remaining hidden cases.
status: addressed
---
