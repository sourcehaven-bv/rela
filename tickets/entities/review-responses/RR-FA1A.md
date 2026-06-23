---
id: RR-FA1A
type: review-response
title: applyServerProperty / buildRelationsBody required when channels disabled
finding: |
  AutoSaveOptions declares both as required. Plan says they're "no-op closures (or omitted when channel is disabled)." Picking "or omitted" requires conditional types in TS and ripples into DynamicForm. Picking "still required, pass () => {}" works but means the type system doesn't enforce the contract. Pseudo-code in the ticket shows no-op closures; commit to it.
severity: significant
status: addressed
resolution: |
  Decided: callbacks stay required at the type level. Document in jsdoc that they are never invoked when their channel is disabled. Add a runtime assertion in mergeServerResponse: when disablePropertyChannel is set, throw if applyServerProperty is invoked (defence-in-depth -- should never fire under correct internal logic). EntityDetail's content-only instance passes `() => {}` for applyServerProperty and `() => null` for buildRelationsBody. Conditional types are a future cleanup if they pay off elsewhere; not warranted now. AC 1 amended to specify "no-op closures required for disabled channels."
---
