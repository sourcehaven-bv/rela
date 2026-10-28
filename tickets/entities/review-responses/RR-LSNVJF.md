---
id: RR-LSNVJF
type: review-response
title: Range is not part of the data shape
finding: Adding min/max could change the datamigration shape hash and force every project with a range to baseline a migration.
severity: minor
resolution: 'min/max stay out of metamodel.PropertyShape: a range is a soft validation constraint that analyze reports, like a regex validation, not a storage shape.'
status: addressed
---
