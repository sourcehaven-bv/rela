---
id: RR-OCKXX
type: review-response
title: No EntityList integration test for AC2/AC3/AC4
finding: 'PLAN-XYB07 calls out: AC2 ''exactly one fetch fires for typing TKT-603'', AC3 ''q=foo in the URL hydrates the search box on mount'', AC4 ''clearing the search restores the unfiltered list''. The vitest tests cover SearchBox and AdHocFilterMenu in isolation. Neither one proves the wiring holds end-to-end in EntityList.'
severity: significant
resolution: 'Added ''EntityList search integration'' describe block with three tests: AC3 (q in URL hydrates input), AC2 (typing 3 chars fires exactly one fetch with q after debounce), AC4 (clear-button removes q from URL and subsequent fetch lacks q).'
status: addressed
---
