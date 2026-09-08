---
id: RR-8JIICP
type: review-response
title: Condition pre-filter and query pushdown gate against two metamodel snapshots
finding: nextActionEngine snapshots st.Meta for the condition prefilters and matchers, while executeQueryPrefiltered gates the query's own pushdown against the read bundle's metamodel resolved separately; a reload landing between them mixes snapshots within one request, against CLAUDE.md's capture-once rule.
severity: minor
resolution: 'Documented at nextActionCandidates'' meta parameter rather than threaded through queryService: the coherence that matters — the condition''s pre-filter and its authoritative Go pass sharing a snapshot — holds (both derive from st.Meta), each pushdown is coherent with its own pass, and a metamodel gate can only decline to push, never widen. Threading the bundle would touch the shared queryService for no soundness gain.'
status: addressed
---
