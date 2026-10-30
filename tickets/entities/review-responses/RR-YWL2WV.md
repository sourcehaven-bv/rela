---
id: RR-YWL2WV
type: review-response
title: AC 10 says no error, but paging failures show one
finding: onJump loads pages through onLoadOlder, which passes any failure to onError, and the page shows it (useChannelPaging.ts:26-36). A message link whose older-page load fails shows an error, which contradicts "does nothing and shows no error" in AC 10. Either narrow the AC to "a seq that is never held" or route link-driven failures somewhere quiet. Write down which.
severity: minor
resolution: 'ADDRESSED AC 10 narrowed: a bad or never-held seq shows nothing, a failed page load while paging for a link shows the channel usual load error; recorded in Q5.'
status: addressed
---
