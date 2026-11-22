---
id: RR-HNQI5JH
type: review-response
title: property-row is also used elsewhere
finding: EntityTypeView uses a .property-row class of its own.
severity: nit
reason: 'Its scoped all: unset wins, so nothing changes there; renaming the shared class touches SectionEditForm and its tests for no visible gain.'
status: wont-fix
---
