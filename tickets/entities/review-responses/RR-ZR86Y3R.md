---
id: RR-ZR86Y3R
type: review-response
title: Wall accepts columns without column_property; wall with column_property undocumented
finding: 'layout: wall with columns: and no column_property passed validation and ignored the columns; wall with column_property was untested in docs.'
severity: minor
resolution: Validation refuses 'columns need column_property' on a plain wall (table case added); docs describe a wall with column_property. Also added kanbans.*.layout to the configedit allowlist, which TestAllowlist_ClassifiesEveryKey required.
status: addressed
---
