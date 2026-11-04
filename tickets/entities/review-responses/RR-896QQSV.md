---
id: RR-896QQSV
type: review-response
title: Singleton page watcher closes over the first consumer
finding: The detached watcher uses the first caller's route-derived computeds.
severity: minor
reason: useRoute needs a component setup, so it cannot move to module level without importing the router instance. The computeds outlive the component on Vue 3.5, and NextActionCard is now mounted at the App root. Revisit if the composable moves to a router-instance source.
status: deferred
---
