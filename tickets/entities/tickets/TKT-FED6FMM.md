---
id: TKT-FED6FMM
type: ticket
title: Remove unpaged list reads from request paths
kind: enhancement
priority: high
effort: l
status: ready
---

## Description

Burn down the `request-path` entries of the unpaged-list allowlist. Replace each
with header reads, store-side paging, `GraphQuery` pushdown or a batched lookup,
and pin each with a row budget test. Split per package (dataentry, visibility,
entitymanager, mcp, appbuild) into follow-up tickets if too large for one PR.
Related: TKT-VJ8FHJF (validator loads candidates per rule).

## Request-path entries (TKT-1VAVZ3O)

The allowlist is `internal/archguard/unpagedread_allowlist_test.go`, keyed per
function. At creation it held 21 `request-path` entries (23 calls):

| Package | Entries | Functions |
| --- | --- | --- |
| dataentry | 5 | `scopedread.go#scopedHeaders` (3 calls, including through `collectHeaders`), `helpers.go#listFromStoreByTypes`, `helpers.go#listAllFromStore`, `helpers.go#visibleListByTypes`, `gantt_handler.go#ganttHandler.ganttEdges` |
| lua | 5 | `runtime.go#Runtime.luaListEntities`, `runtime.go#Runtime.luaGetRelations`, `elevation.go#elevatedListEntities`, `elevation.go#elevatedGetRelations`, `markdown.go#mdEntityRefs.luaMdEntityRefs` |
| entitymanager | 3 | `unique.go#checkUniqueProperties`, `core.go#collectAllIDs`, `rename.go#idTakenByOther` |
| search | 2 | `index.go#Service.listAll`, `index.go#Service.scanFaces` |
| visibility | 2 | `holders.go#HolderLookup.unrestricted`, `headerquery.go#HolderLookup.headersFromList` |
| appbuild | 1 | `idproles.go#roleHolders.PrincipalsWithRole` |
| desktopnotify | 1 | `evaluate.go#Config.Evaluate` |
| ingest | 1 | `ingest.go#findTarget` |
| mcp | 1 | `tools_entity.go#Server.handleListEntities` |

## Done when

- No `request-path` entries remain in the allowlist.
