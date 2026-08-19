---
id: RR-BL8OW8
type: review-response
title: 'direction: "" bypasses the ambiguity check entirely'
finding: 'Direction.UnmarshalYAML mapped the empty string to DirectionOutgoing (`case "", "outgoing"`). The whole change assumes "absent means empty in Go", which holds only for a MISSING key. A written-but-empty `direction: ""` unmarshalled to "outgoing" and so walked straight past the self-referencing ambiguity check — the entire point of the ticket. Confirmed against the real ValidateConfig: an ambiguous binding with direction: "" validated clean. Also, on a to-side binding, direction: "" produced a confusing wrong-side error instead of inferring incoming. This is not hypothetical: `direction: {{ .dir }}` with an unset variable, or any config generator, emits exactly this.'
severity: critical
resolution: 'Fixed in internal/dataentryconfig/config.go: UnmarshalYAML no longer collapses "" to outgoing — an empty value now stays empty so the single InferDirection rule owns the decision. Updated TestDirection_UnmarshalYAML (the case that pinned the old collapse) and added a bare-key case. New regression test TestValidateConfig_EmptyDirectionStringIsNotOutgoing covers both arms: ambiguous still errors, to-side still infers incoming.'
status: addressed
---
