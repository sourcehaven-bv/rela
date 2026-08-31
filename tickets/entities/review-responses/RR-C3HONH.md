---
id: RR-C3HONH
type: review-response
title: SPA never passed ?root=, so drill-into-truncated-subtree could not work
finding: getGantt(props.id) was called with no root argument and drilling was entirely client-side over the already-fetched forest — so when `truncated` was set, drilling into a cut-off subtree found nothing, while the truncated tooltip literally promised 'drill in to see more'. The server's ?root= contract (uniform 404, dedicated test) had no caller.
severity: significant
resolution: 'GanttView now tracks fetchedRoot and applies a fetch policy: drilling stays client-side when the fetched, untruncated tree can answer (fast path, no refetch); it refetches with ?root= when the data was truncated or the target is missing; going back above a scoped fetch refetches likewise. Breadcrumb titles are remembered per drill so a re-scoped response that lacks ancestors still labels the trail. Pinned by 5 new component tests in GanttView.test.ts (mount fetch, client fast path asserts NO refetch, truncated→?root= refetch, back-above-scope refetch, error rendering).'
status: addressed
---
