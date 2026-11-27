---
id: RR-BHWO42B
type: review-response
title: Default traversal SQL could not use the derived index
finding: 'The OR with the empty test did not imply the partial index predicate, so ACL when: gates on a default value scanned the type.'
severity: significant
resolution: New derived empty index (store.DerivedEmptyIndex) built by pgstore; the EXPLAIN test checks a defaulted case uses it.
status: addressed
---
