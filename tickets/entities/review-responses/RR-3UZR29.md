---
id: RR-3UZR29
type: review-response
title: Impression reported on load rather than on render
severity: significant
status: addressed
finding: 'The impression that starts a cooldown was reported from load(). Since load() also runs after feedback, snoozing suggestion A immediately marked its replacement B as shown -- starting B s 24h cooldown for an impression that never happened, so B was suppressed having never appeared. Directly contradicted the stated invariant that a discarded response cannot silently consume a suggestion.'
resolution: 'load() no longer reports. A markShown() function is called by the component that actually renders the suggestion -- the page card for banner/notice, the status-bar chip for its own tier -- and is idempotent per suggestion key so the two shared surfaces cannot double-count. Four tests cover it, including that a replacement gets its own impression once rendered.'
---
