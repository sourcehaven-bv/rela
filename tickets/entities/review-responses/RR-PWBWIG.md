---
id: RR-PWBWIG
type: review-response
title: Docs describe the earlier-entry check per poster of the entry, not the implemented rule
finding: 'docs-project/entities/guides/GUIDE-realtime.md:909 (and docs/realtime.md) says rela checks earlier entries ''for each poster the entry admits''. The implementation records every earlier person-fired entry, whether or not it admits a poster of the button''s entry (the documented deviation for two-role callers). For an earlier entry whose when fails to evaluate but admits none of the button''s posters, it hides the button instead of refusing it (buttons.go:272-275). Line 916 documents only the refusal half. An operator reading the docs cannot predict why a member+lead caller loses a member-only button. Fix: say that every earlier entry that could take the click is judged, and that a caller holding any role it admits does not see the button. Add one sentence for the error case: refused when the entry admits one of the button''s posters; otherwise the button is hidden from the callers that entry admits.'
severity: minor
resolution: 'Rewrote the earlier-entry bullet in the guide: rela judges every earlier person-fired entry, a caller holding any role it admits loses the button, with a lead+member example. Added a bullet for the error case: refused when the entry admits a poster of the button entry, otherwise hidden from the callers it admits. docs/realtime.md regenerated with just docs.'
status: addressed
---
