---
id: view-enum-labels-test
type: automated-measure
title: 'Test: view sections show enum labels when the property name differs from its type'
description: Guards BUG-FY36QZ. The e2e spec renders a single and a list enum in a custom view display section, with property names that differ from their enum type names, and asserts the schema labels. Unit tests pin the same for the schema-def and routing-hint paths of PropertyDisplay, inline enums, and that a type declaring the property never borrows another type's labels.
kind: test
location: e2e/tests/view-enum-labels.spec.ts; frontend/src/components/common/PropertyDisplay.test.ts; frontend/src/stores/schema.test.ts (does not borrow another type's labels)
status: active
---
