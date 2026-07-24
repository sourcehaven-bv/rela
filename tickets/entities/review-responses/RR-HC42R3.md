---
id: RR-HC42R3
type: review-response
title: list_entities materializes the full type set before filtering — unbounded on large graphs
finding: The plan notes materializing iterators is 'bounded by existing Lua-table construction'. That holds for get_entity/search (search has an explicit limit) but rela.list_entities(type) has NO limit parameter — it streams every entity of a type into a Lua table today, and would now additionally build a Go slice of the same size for Filter, then a redacted copy per survivor. On a large graph that is up to 3x the peak allocation of today's path. Not a correctness bug and arguably pre-existing, but worth either batching Filter in chunks or noting the multiplier honestly in godoc rather than claiming 'no new unboundedness'.
severity: minor
resolution: 'Plan corrected: the ''no new unboundedness'' claim is dropped. list_entities has no limit, so Filter adds a Go slice + per-survivor redacted copies over today''s Lua table — up to ~3x peak allocation on a large type. Implementation either chunks the Filter or states the multiplier honestly in godoc.'
status: addressed
---
