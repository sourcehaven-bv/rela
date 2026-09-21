---
id: RR-TOUCHW
type: review-response
title: userTouched is set unconditionally, so a duplicate 403s where a fresh create would succeed
finding: '"applyEmbeddedPrefill marks every prefilled key userTouched. visibleWritablePropertiesForCommit short-circuits on userTouched BEFORE both the stagedVisibleProps and writable===false checks, so a duplicate submits property keys that a fresh create of the same type, by the same principal, would omit. The prop doc argues the server''s affordance gate is the real boundary and refuses loudly with a rule_id — which is the right outcome for a field the user TYPED INTO (RR-2U2D''s actual rationale) but converts a working create into a 403 for a field they never saw. The security review confirmed the server does deny these (RuleFieldHidden / RuleFieldReadOnly, 403 + audit row), so this is a usability regression rather than an escalation, and confirmed the value itself cannot leak (stripHiddenProperties already emptied hidden fields from the source)."'
severity: significant
resolution: Accepted as-is, with the scope written down. The security review confirmed the server denies these (RuleFieldHidden / RuleFieldReadOnly, 403 + audit row) and that a value for an unreadable field cannot be constructed, since stripHiddenProperties already emptied the source. Narrowing to post-dry-run would trade a loud 403 for a silent drop, which is the failure this ticket exists to avoid. Recorded that RR-2U2D's rationale is being extended past its stated scope.
status: addressed
---

## Suggested resolution

Narrow the marking to keys present in stagedVisibleProps with writable !== false once the first dry-run resolves, or write down explicitly that RR-2U2D's rationale is being extended past its stated scope.
