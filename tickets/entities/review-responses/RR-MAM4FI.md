---
id: RR-MAM4FI
type: review-response
title: 'Existing in: bullets in the guide now read as absolute'
finding: 'GUIDE-realtime.md:777-778 still says "A stranded session admits only the types without `in:`", and :779-783 says the trigger warning "names the trigger and the `in:` key". Under default_in: keyed, a type with no in: in the file is refused in a stranded session, and the warning names session.default_in. The new section corrects both points, but 20 lines further down, so a reader of the earlier bullets gets the wrong rule. Fix: change them to "the types without `in:`, written or taken from `session.default_in`" and "names the trigger and the rule (the type''s `in:` or `session.default_in`)", then run just docs.'
severity: minor
resolution: 'The stranded bullet now says "the types without in:, written or taken from session.default_in" and the trigger bullet says the warning "names the trigger and the rule (the type''s in: or session.default_in)"; just docs regenerated docs/realtime.md.'
status: addressed
---
