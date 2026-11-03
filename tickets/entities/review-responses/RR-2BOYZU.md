---
id: RR-2BOYZU
type: review-response
title: 'The guide says notify: false stops what it cannot stop'
finding: 'docs-project/entities/guides/GUIDE-realtime.md:236 says "Mentions and replies in them still reach the inbox; use `notify: false` on a property for that." notify: false on a ref property only stops mentions from that property. A reply row is written for every reply, and no key stops it. Reword: "Mentions and replies in them still reach the inbox. To keep a ref property from mentioning anyone, use `notify: false` on it." Regenerate docs/realtime.md.'
severity: minor
resolution: ADDRESSED guide reworded as suggested; docs/realtime.md regenerated.
status: addressed
---
