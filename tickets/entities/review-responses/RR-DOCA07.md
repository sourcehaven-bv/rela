---
id: RR-DOCA07
type: review-response
title: because= matched by unanchored substring, so a single character satisfied it
finding: because="a" passed against the reason 'no role grants update on type risico'. The argument exists to pin WHY a decision came out as it did; a one-character match pins nothing while looking like it pins something.
severity: significant
resolution: The rule identity (RuleKind, RuleID, or kind/id) must match exactly; the free-text Reason keeps substring matching with a minimum fragment length, since a manual should be able to quote a meaningful phrase rather than a whole sentence.
status: addressed
---
