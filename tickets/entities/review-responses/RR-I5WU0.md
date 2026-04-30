---
id: RR-I5WU0
type: review-response
title: AdHocFilterMenu in list mode unnecessarily depends on schema store
finding: useSchemaStore() is imported and called at component setup. In list mode (where entityType is provided), the store is only consulted in code paths that should not run in list mode. Producer-side coupling to the store couples list mode to search mode needs.
severity: significant
reason: Decoupling AdHocFilterMenu from the schema store would mean passing in a valueResolver callback per call site. Current coupling is acknowledged as a smell but the refactor is not blocking — both modes work correctly. Filed as a follow-up consideration. The mode prop introduced for C4 already separates the two behaviors enough that the schema-store coupling only fires on the search-mode codepath in practice.
status: deferred
---
