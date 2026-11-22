---
id: RR-HW0TU97
type: review-response
title: Dashboard table cards kept raw property names
finding: DashboardView headed a column without a label with the raw property name while lists and views now use the schema label.
severity: significant
resolution: getColumnLabel falls back to the schema label of the first row type; a DashboardView test covers it.
status: addressed
---
