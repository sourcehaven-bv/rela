---
id: BUG-FY36QZ
type: bug
title: Enum fields in custom view sections show raw values instead of labels
description: 'A display section (source: entry, display: properties) in a custom view renders an enum field as a badge with the raw value (e.g. external_obligation) instead of the label declared on the enum type (External obligation). The edit form for the same entity shows the labels. Cards and list sections have the same defect. It shows when the property name differs from its enum type name, e.g. property kind of type task_kind.'
priority: medium
effort: s
why1: The display widget gets the enum TYPE name (the wire field propType) as its propertyName, and no entity type. Badge resolves labels with getEnumLabel(value, property, entityType), which looks for a property with that name. No property is called task_kind, so it returns nothing and the raw value shows.
why2: PropertyDisplay (entry sections) and viewFieldRoutingHint (cards/list sections) both pick propType ?? property as propertyName. propType was added to the view wire shape for badge colours, and the server keys styles by type name, so passing the type name made colours work.
why3: Enum labels were added later through the same propertyName channel, but labels are resolved per property (inline enums keep labels on the property, custom types on the type). The view paths were not updated; forms, tables and kanban already passed the real property name plus entity type.
why4: 'One prop (propertyName) carried two meanings on view surfaces: a style key on some call sites and a property name on others. stylesForProperty grew a direct-key fallback to accept a type name, which hid the mismatch instead of removing it.'
why5: No test rendered a view section enum whose property name differs from its type name; the e2e fixture used status of type feature_status with no labels, so the type-name shortcut was never exercised against labels.
prevention: Pass the real property name and entity type on every display surface; stylesForProperty already maps a property to its type for colours. The new e2e spec renders a single and a list enum whose names differ from their types in a view section and asserts the labels.
started: "2026-10-06"
completed: "2026-10-06"
status: done
---
