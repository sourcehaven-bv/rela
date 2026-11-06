---
id: RR-DYEE08G
type: review-response
title: Relation meta check passed when the source row was missing
finding: relationSources returns a nil row when the source cannot be read and validateRelationMetaWrite then allows the write.
severity: minor
resolution: authorizeConflictFields answers the uniform conflict 404 when any source row is nil. The nil-side dereference is also gone (sideProperties).
status: addressed
---
