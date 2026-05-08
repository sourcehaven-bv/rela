---
id: RR-R51D0
type: review-response
title: Don't blank results during in-flight refetch (flicker)
finding: 'Plan doesn''t specify the visual state during a debounced/in-flight refetch. If we clear results on every keystroke and only refill on response, the listbox flickers between empty and populated — palette feels stuttery. Required: in Approach, keep previous results visible during in-flight refetch; render a subtle loading indicator (small spinner in the input) instead of blanking the list. Clear results only on (a) new response arriving (replace) or (b) query becoming empty. Test: type ''a'', let response settle; type ''b'', advance timers but don''t resolve the second request — assert previous results still rendered and a loading hint visible.'
severity: minor
resolution: 'Plan updated: results.value is replaced only inside the success branch of the try block — the previous results stay rendered while a new request is in flight. A loading indicator (loading.value) is rendered alongside. Test added: type ''a'', resolve; type ''b'', advance timers but don''t resolve; assert previous results visible and loading=true.'
status: addressed
---
