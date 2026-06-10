---
id: RR-TLT9CR
type: review-response
title: _position source=search bypasses the read gate — leaks hidden total + neighbor IDs/types
finding: 'resolveScope''s source=search branch called executeQuery (ungated) directly: handleV1EntityPosition then served Total from the unfiltered set and shipped hidden {ID, Type} pairs in prev/next, letting a denied principal walk the hidden set neighbor-by-neighbor and read exact hidden cardinality — the precise leak shape TKT-VMD8 closes on the list path. Also made the docs claim (''_position shares the gated list pipeline'') false for search scopes.'
severity: critical
resolution: 'Added App.readableSubset (scope.go): filters the search result through the readGate before returning, batching PermitsReadMany per entity type (mirrors the TKT-VQGN include-filter batching rule); gate errors wrap errACLListQuery and route through writeGateError. Pinned by TestACLPosition_SearchScopeGated: hidden FEAT must not count in total, must not appear in prev/next, and a hidden id 404s out of the scope. GUIDE-acl-security updated to state both scope sources are gated. Commit 622b6cf7.'
status: addressed
---
