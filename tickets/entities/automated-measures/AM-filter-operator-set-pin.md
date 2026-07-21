---
id: AM-filter-operator-set-pin
type: automated-measure
title: 'Test pin: config validator accepts exactly the documented list-filter operator set'
description: TestValidateConfig_DocumentedFilterOperatorsAccepted iterates the documented operator set (docs/data-entry.md "Static Filters") and asserts each validates, while TestValidateConfig_InvalidFilterOperator asserts `=~` (the cross-language confusion that produced BUG-F1LTV0) is rejected. Prevents the validator's operator set from drifting away from the docs/SPA/API again.
kind: test
location: internal/dataentryconfig/validate_test.go
status: active
---
