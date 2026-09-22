---
id: RR-2TVXO7
type: review-response
title: resolveCommands filter and handleCommandExec enforcement must call the SAME authorizer instance
finding: 'Today resolveCommands (presentation: omit buttons) and handleCommandExec (enforcement: 403) both call the single free function authorizeCommand, so they cannot drift — DEC-EIHQSU explicitly calls this out (''the SINGLE decision point, called by both''). The plan keeps one commandAuthorizer on commandHandler, which preserves this, but it should be stated as an invariant so a future refactor doesn''t give the filter and the exec path different authorizers (which would render buttons that 403, or hide buttons that would run).'
severity: minor
resolution: 'Plan updated to pin the invariant: both resolveCommands and handleCommandExec call h.authz.Authorize — one field, one instance, never re-derived per call site. Kept the DEC-EIHQSU framing (''resolve filter is UX; Authorize at exec is the boundary''). No separate test needed beyond AC2 (button omitted) + AC2''s exec-403 pairing, which together prove they agree.'
status: addressed
---
