---
gated_by: [acl]
---
queryplan lowers the store-safe part of a query. The caller combines it with
the `store.GraphQuery` that `acl` compiles for the request, so the store
applies the row gate in the same query.
