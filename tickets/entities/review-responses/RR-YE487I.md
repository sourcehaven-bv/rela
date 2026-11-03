---
id: RR-YE487I
type: review-response
title: View model cannot express TKT-03FQPD or FPXTQJ enum choices; 'not blocked' claim is too strong
finding: 'Open question 10 says RlChannelDecisions ''takes plain views and emits a name, so message options can render through it''. TKT-03FQPD posts ''the declared answer type with that choice'', and has an ''other'' option that opens free text. TKT-FPXTQJ''s motivating case is one `plan decision: approve | revise` enum type whose guarded edges branch. Both need a button = (message type, preset props), and several buttons for one type. ChannelDecisionView is keyed by messageType, emits `decide: [messageType]`, and pendingKey is ''decision:''+type, so two buttons of one type collide. Also, composerTransitions builds a Map by message_type, so with FPXTQJ''s several edges per type it keeps the last edge, and toLabel/confirm/tone come from the wrong one. Cheap now, a breaking library API change later: give the view an opaque `key` (emit and pending by key), an optional `props` preset, and derive opensForm when any edge of the type is guarded and toLabel only when the target is unique.'
severity: significant
resolution: ChannelDecisionView now has an opaque key (emitted and used for pending) and optional preset props, decisionViews groups all edges per type, sets opensForm when any edge is guarded and toLabel only for a single target, and confirm only when every target is final.
status: addressed
---
