---
id: RR-IRLQ7
type: review-response
title: Label/help/error rendering ownership undefined
finding: Today FieldRenderer renders label, .field-help, .field-error, .form-field layout. After extraction it is unclear if widgets render these (9 inconsistent implementations) or a wrapper does (then the wrapper IS the new FieldRenderer).
severity: significant
resolution: 'Plan revised: new FieldShell.vue owns label/help/error/layout/labelPosition. Widgets render only the input control. FieldRenderer.vue becomes thin glue wrapping the resolved widget in FieldShell. See TKT-MZSIJ ''Label/help/error ownership''.'
status: addressed
---
