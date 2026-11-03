---
id: RR-V6K0FZV
type: review-response
title: Compact CSS overrides the container query
finding: 'Planned compact selectors (specificity 0,2,0) beat the single-class rules inside @container (max-width: 32rem) in RlChannelSession.vue:340-393. The strip would show below 32rem and the list would never unfold in compact at narrow widths.'
severity: significant
resolution: 'Plan fixes the selectors: the list hide uses :not(.rl-channel-session__steps--expanded); the strip hide inside the container query uses the same compact-class specificity so source order decides. Playwright checks at 24rem.'
status: addressed
---
