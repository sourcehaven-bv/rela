---
id: RR-8AUK7
type: review-response
title: Modifier-click and middle-click behavior worth documenting
finding: '<a :href :click.prevent> + browsers handle ctrl/cmd/middle-click natively before JS runs, so right-click / new-tab / new-window all work via href. Plan should note: do NOT add defensive modifier checks in the click handler. Bonus: consider <RouterLink :to> which handles all of this for free. If we explicitly choose plain <a>, document why.'
severity: nit
resolution: Plan uses plain @click.prevent without modifier checks. Browsers handle ctrl/cmd/middle-click natively via href before JS runs. Considered <RouterLink :to> but plain <a> with helper is simpler and matches the rest of the SPA's pattern.
status: addressed
---
