---
id: RR-MPE9Y
type: review-response
title: Whitespace error doesn't list available properties; doc claims it does
finding: 'GUIDE-metamodel.md promises: "A typo or whitespace mistake fails metamodel-load with a clear diagnostic naming the entity, the offending value, and the available properties." But validateDisplayProperty''s whitespace branch (loader.go) lists no properties — only the missing-property branch does. Author with display_property: " titel " (and a typo too) gets the whitespace error, fixes it, re-runs, then sees the missing-prop error. Two round-trips. Fix the whitespace branch to include the same available-properties list, OR update the doc to match. Symmetric error is the better fix and dovetails with the accumulating-style request below.'
severity: significant
resolution: 'Whitespace branch now appends ''(have: <props>)'' just like the missing-property branch, listing every defined property name. Test asserts the diagnostic includes ''display_property'', ''whitespace'', and the actual property name ''titel'' so an author with both whitespace and a typo can fix both in one round.'
status: addressed
---
