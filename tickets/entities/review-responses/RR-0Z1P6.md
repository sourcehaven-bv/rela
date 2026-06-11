---
id: RR-0Z1P6
type: review-response
title: Default widget table loses multi-axis decision logic
finding: 'Today''s FieldRenderer dispatches on type AND propertyDef flags: list:true => multi-select, values.length>0 => select, boolean => checkbox, etc. Flat propertyType => widget table can not express this. ''No behaviour change'' claim is therefore false as written.'
severity: significant
resolution: 'Plan revised: defaultWidgetFor takes full PropertyDef (not propertyType string) and encodes the multi-axis order matching FieldRenderer today (list -> multi-select, values.length>0 -> select, etc.). Snapshot test verifies parity. See TKT-MZSIJ ''Default widget resolution''.'
status: addressed
---
