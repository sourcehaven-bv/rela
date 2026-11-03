---
id: RR-VX2G41
type: review-response
title: pageTabView copies PageView's tab rule
finding: useViewEntityTypes.pageTabView repeated how PageView picks the open tab from the route (anchorId/tabId), so a change to PageView would order the menu by a tab that is not on screen.
severity: significant
resolution: Extracted routeTabId into utils/pageContext.ts; PageView and pageTabView both call it; unit tests in pageContext.test.ts.
status: addressed
---
