---
id: RR-2OJLOV
type: review-response
title: Exported Offers trusts that t belongs to st
finding: 'internal/realtimecfg/branches.go:62 indexes st.on[t.MessageType] by t.index. A Transition from another state returns a silently wrong answer. Fix: document ''Nil/foreign: t must come from st.Transitions or st.Branches'', or check that st.on[t.MessageType][t.index].key == t.key and return false otherwise.'
severity: nit
resolution: Offers now returns false when t is not the entry at its index in this state (key mismatch or out of range), with a Foreign t contract in its doc comment. Added TestButtons_OffersForeignEntry.
status: addressed
---
