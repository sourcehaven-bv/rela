---
id: RR-P0OPHN
type: review-response
title: Fallback relation write takes the write lock for every type
finding: The data-entry fallback wrapped every create in store.Tx, not only owning ones; the 422 mapping was untested.
severity: minor
resolution: Tx only for owning types. TestOwner_FallbackWriteAppliesOwningRules, mutation-checked.
status: addressed
---
