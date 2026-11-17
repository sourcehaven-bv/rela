---
id: RR-1R7HZC9
type: review-response
title: Schema field removal locks old rows
finding: Update re-validated the merged stored fields, so a field removed from the schema made every old row unchangeable.
severity: significant
resolution: Update drops stored fields the schema no longer declares before validation; TestUpdate_DropsUndeclaredStoredField.
status: addressed
---
