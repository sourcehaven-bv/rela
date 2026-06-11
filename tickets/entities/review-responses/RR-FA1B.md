---
id: RR-FA1B
type: review-response
title: mergeServerResponse must still update lastSeenServer for a disabled property channel
finding: |
  Plan says "skips the iteration for the disabled channel." For property channel that's wrong: skipping lastSeenServer updates means a later re-enable observes a stale baseline. If we keep the update, the disable is effectively "skip applyServerProperty callback" not "skip the iteration".
severity: significant
status: addressed
resolution: |
  Plan revised: disablePropertyChannel skips the applyServerProperty CALLBACK invocation and the disappeared-key cleanup; it still updates lastSeenServer[k] = v from the server response. Same shape for disableContentChannel: skips applyServerContent but keeps lastSeenContent = entity.content. Re-enabling a channel mid-session is explicitly NOT supported in this ticket (declared unsupported with a brief jsdoc note); the lastSeenServer maintenance is preserved for forward-compat. AC 1 amended to clarify "channel disable skips callbacks, not baseline tracking."
---
