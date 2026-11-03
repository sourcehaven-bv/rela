---
id: RR-RF6QPI
type: review-response
title: Wire split rationale is wrong; keep the split but fix the reason and pin its invariant
finding: 'Approach 6 (plan lines 355-358) says a union served as boolean-or-text would make false ambiguous with absent. It would not: absent, false, true and "text" are four distinct JSON states; the ambiguity only comes from Go omitempty on a bool, which a *bool or a custom MarshalJSON avoids. The split is still the better wire (no oneOf in OpenAPI, one boolean plus an optional string in TS), and it differs knowingly from an action''s union confirm (dataentryconfig config.go:379-387), so say that instead. Pin the invariant in TestSessionView_ButtonOptions and the OpenAPI description: confirm_text present implies confirm is true, and confirm false never carries a text. Name the Go Confirm enum so its zero value is ConfirmDerived.'
severity: minor
resolution: Approach 8 now gives the true reason for the split (one boolean plus an optional string, no oneOf) and notes it differs knowingly from an action union. The invariant is pinned in TestSessionView_ButtonOptions and the OpenAPI description, and the enum is ConfirmMode with ConfirmDerived as zero value.
status: addressed
---
