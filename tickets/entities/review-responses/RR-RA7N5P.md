---
id: RR-RA7N5P
type: review-response
title: Route name used as view kind
finding: The non-page branch passed the route name in as the view kind; a renamed route would silently drop the feature.
severity: minor
resolution: Explicit routeViews map from route name to PageTabView.
status: addressed
---
