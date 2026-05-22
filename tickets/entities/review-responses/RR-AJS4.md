---
id: RR-AJS4
type: review-response
title: Value constructors don't copy inputs; document or copy
finding: 'value.go NewRecord(map) and NewList(slice) hold their inputs by reference. The Elems() doc says ''callers must not mutate'' (the receiving side); the constructing side has no such warning. A caller mutating the map/slice after passing in produces undefined behaviour. Pick one: (a) document explicitly on NewRecord/NewList that the engine takes ownership; (b) copy at construction. (a) is the cheap pragmatic choice; (b) is safer for the public API.'
severity: minor
resolution: 'Documented ownership transfer on NewRecord and NewList: ''The returned Record/List retains the supplied map/slice by reference — callers must not mutate it after the call.'' Chose option (a) document over (b) copy to avoid the per-allocation cost on a hot path; the contract is now explicit and matches the receiving side''s Elems() doc.'
status: addressed
---
