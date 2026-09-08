---
id: RR-BYK5TO
type: review-response
title: 'PR-C significants: godoc misfile, torn-count probe, order-dependent mismatch finding, summary blindness'
finding: (1) validateRelationScope was inserted between validateRelationOrderable's godoc and its declaration, silently re-attributing the orderable/symmetric rationale; (2) the boot warning's two CountEntities probes are non-atomic against a live multi-writer store (pg listener starts in Open) — a concurrent create can suppress the warning, and the comment claimed a cost decision without acknowledging the torn read; (3) the state-type-mismatch check ran during the streaming pass and silently depended on default-before-state iteration order that no backend documents — a conforming new backend could drop the finding class; (4) analyze all reported state findings in the text section but the summary box and the JSON envelope carried nothing — a JSON consumer was blind to the ticket's headline finding (the JSON path returns before sections run).
severity: significant
resolution: (1) moved below validateRelationOrderable with its own godoc; (2) comment now states the probe is advisory and non-snapshot with the authoritative report being `rela analyze states`, plus Debug traces on probe errors; (3) the type comparison moved into the post-collection loop over completed families — the ordering dependency is gone by construction, and mismatches aggregate per family; (4) Summary.States + the `states` JSON key + totalIssues + the summary box all carry the count (AnalyzeAll now runs CheckStates; the pre-existing double-scan shape is commented like cardinality's).
status: addressed
---
