---
id: RR-S4S8ZG
type: review-response
title: Family and FamilyMany load full bodies for existence checks
finding: Family reads full rows with AllStates although its callers need only id/type/face; on the FilterRelations list path this breaks the content-free batched collection-read rule
severity: significant
resolution: 'Design section 8.7 (PR 2 and PR 5): Family reads ListEntityHeaders and returns readable faces sorted by token; content via Ref on the chosen face. EndpointsReadable is header-only and batched with a storetest.Counting budget test at 10 and 50 relations.'
status: addressed
---

**Where:** design section 2, `Family` and `FamilyMany`, and the Loader.

Family mode loads full rows with `ListEntities{IDs, AllStates}`. Its main
callers only need existence, type and face: delete/rename 404 decisions,
relation endpoints, `_faces` enumeration, and `FilterRelations` on list paths.
CLAUDE.md requires collection reads to be content-free and batched, with a
`storetest.Counting` budget test. Loading every face's body per relation
endpoint on a list is the cost that rule exists to prevent. `Family.Faces` is
also described as "store order", which is not deterministic on fs/mem.
