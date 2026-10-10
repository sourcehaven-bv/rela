---
id: RR-W0SOYZ
type: review-response
title: sectionId union in KanbanView
finding: sectionId(string | Section) exists because RlBoard emits a Section and RlSwimlaneBoard an id.
severity: nit
reason: The two boards already differ in their expandSection payload; collapseSection follows each board's existing convention so callers handle expand and collapse the same way. Changing the emit shapes across the library is out of scope, and one small helper is clearer than four handlers.
status: wont-fix
---
