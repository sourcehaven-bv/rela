---
id: RR-BXSM9E
type: review-response
title: Unknown keys, nil values and empty strings widen get_relations to the whole graph
finding: '{form=id}, {from=nil-valued-expr} and {from=''''} all became an unfiltered query that the script reads as filtered.'
severity: significant
resolution: relationQueryArg now rejects unknown keys, empty strings and an empty table; the whole graph is only available as get_relations() with no argument. Each case is in TestLuaReads_RejectBadArguments.
status: addressed
---
