---
id: RR-UVJOFV
type: review-response
title: filter.matchList kept a duplicate copy of the empty-list rule
finding: internal/filter/match.go intercepts list-valued properties and routes to matchList BEFORE the propmatch delegation, and matchList carried its own copy of the empty-list rule (len(list)==0 -> match only on OpEqual with empty value). That is exactly the duplicated notion of emptiness propmatch was created to eliminate, sitting one file away. It agreed with propmatch at the time of review, but nothing pinned that it would stay agreeing.
severity: minor
resolution: matchList's empty-list case now delegates to propmatch.Decide for OpEqual/OpNotEqual, so there is one definition. Also documented in propmatch's package doc that filter intercepts lists before delegating (it has richer per-operator list handling for regex/fuzzy), so only the empty-list case reaches Decide from that caller -- while the store backends route every shape through it. The storetest Props_value_shapes case pins both against each other.
status: addressed
---
