---
id: RR-U57RKZ2
type: review-response
title: Datamigration check reads property names as type names
finding: rename_property from/to are property names, so a legacy _x property cannot be renamed.
severity: minor
reason: Property names starting with _ are reserved for rela too (order properties), so refusing them is intended.
status: wont-fix
---
