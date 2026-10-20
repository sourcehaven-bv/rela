---
id: RR-97M1DW
type: review-response
title: loadOlder ignores MAX_HELD_MESSAGES; live merge then evicts what the user is reading
finding: 'useChannel.ts:237-256 loadOlder prepends without a cap, so scrolling back grows the list without bound. The next live message goes through merge() (148-152), which slices to the newest 500. That drops exactly the old messages the user just loaded and is reading, and resets nextBefore, so the scroll jumps. Apply one policy: cap in loadOlder too, and when the user has loaded history, evict from the end the user is not looking at, or suspend eviction while scrolled up. Test loadOlder past the cap, followed by a live message.'
severity: minor
resolution: loadOlder now caps the list by dropping the newest messages and sets hasNewer, live merges do not append or evict while hasNewer is set, loadNewest returns to the live end, and a test covers loadOlder past the cap followed by a live message.
status: addressed
---
