---
id: RR-LJN62L
type: review-response
title: Indicator text reorders on every refresh and is re-announced in the shared live region
finding: 'Step 7 sorts typingNow by at, and at is set to now on every refresh, so with two typists the order alternates ("Ann and Bo" then "Bo and Ann") as each refreshes. The DOM text changes every few seconds. Step 17 puts the text in the existing aria-live="polite" ephemeral region (packages/rela-components/src/components/channel/RlChannel.vue:324-329), so screen readers re-announce it repeatedly. Plan change: sort by the carried-over start time (or by name), and render typing in its own element with aria-live="polite" and aria-atomic="true", apart from operator signals, so it is announced only when the set of typists changes. The RlChannel Typing story should assert that a refresh with the same typists does not change the text.'
severity: minor
resolution: 'Plan updated: entries sort by since then author user; since is kept across refreshes; the text renders in its own aria-live polite, aria-atomic element; the Typing story asserts unchanged text.'
status: addressed
---
