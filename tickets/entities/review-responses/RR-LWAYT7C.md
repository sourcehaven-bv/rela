---
id: RR-LWAYT7C
type: review-response
title: 'Code: cost and cap tests never clear a hidden value'
finding: The fixture set(id; prop; nil) kept the key with a nil value which the store counts as set; so the budget and row-cap tests compared two runs with the same pending rows and could not catch a cost or refusal that depends on hidden values.
severity: significant
resolution: set deletes the key for nil. Elements no longer sends per-row hidden fields to the store at all and TestElements_NeverAsksStoreWhichRowsHoldHiddenValues pins that with a scope spy.
status: addressed
---
