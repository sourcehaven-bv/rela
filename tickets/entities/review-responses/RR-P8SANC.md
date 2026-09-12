---
id: RR-P8SANC
type: review-response
title: wizard.goTo(0) fires router.replace under an active onBeforeRouteLeave guard
finding: goTo → writeStep (useFormWizard.ts:255-262) calls router.replace, and onBeforeRouteLeave (:1770) fires on any navigation including a same-path query change. If goTo(0) ran while dirty were still true the reset would prompt 'unsaved changes' by itself. The current ordering (dirty cleared at :1178, before goTo) survives, but this is load-bearing ordering the plan does not name. Pin it in a comment and test that the guard does not fire, not just the query value.
severity: minor
status: addressed
resolution: >-
  AC-9 extended to assert the leave-guard does not fire on goTo(0)'s router.replace, and the dirty-before-goTo ordering is called out as load-bearing.
---
