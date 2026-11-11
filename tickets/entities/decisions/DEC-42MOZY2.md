---
id: DEC-42MOZY2
type: decision
title: Direct messages are a rela-managed _dm entity with a channel
context: Every realtime channel hangs off an entity and gates on reading it. A DM has no natural entity. An operator-declared conversation type would make privacy depend on ACL authoring; an anchor outside the graph would rework the whole channel subsystem.
consequences: The channel machinery is reused unchanged. DMs need person mapping in acl.yaml. The _dm header is a graph entity (audited) but readable only by its participants; DM existence stays secret through the uniform 404. Operator access can later be added as one more read grant without changing the model. Group DMs would need a different id scheme.
date: "2026-11-10"
status: accepted
---

## Decision

Direct messages are 1:1 conversations between two people. Each is a rela-managed
`_dm` entity, in the style of `_flow-run`, with two managed `_member` edges to
the participants' user entities. A managed `_direct` channel type with
`cardinality: one` hangs off it. The `direct:` block in `realtime.yaml` turns
DMs on and declares their message types.

Only the two participants can read a `_dm` and its channel. Only
`system:realtime` writes `_dm`. The id is a hash of the sorted pair of user
entity ids.

## Out of scope for now

Group DMs, agents as participants, operator read access.

Discussion and alternatives: TKT-55RNWWD.
