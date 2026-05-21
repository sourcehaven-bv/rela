// Dev-mode diagnostic for the `_actions` affordance field.
//
// The data-entry server always emits `_actions` on entity / list
// responses (see internal/dataentry/affordances.go). When a
// development build receives a response from a whitelisted endpoint
// that omits the field, that's a server-side regression worth
// flagging at the edge. Production builds emit no warnings — silent
// fallback to "render all controls" (the server still 403s on click).
//
// Dedup: a module-level Set of seen request paths keeps the warning
// from flooding the dev console on SSE refresh / cache invalidation.
// HMR clears the Set so a code change during dev re-arms the warning.

interface AffordanceCarrier {
  _actions?: Record<string, boolean>
}

const seen = new Set<string>()

/**
 * Warn (once per requestPath, in dev only) if the response is missing
 * the `_actions` field. Use only on entity / list endpoints; do not
 * call from search, analyze, SSE, or any non-entity endpoint.
 */
export function warnIfMissingActions(
  response: AffordanceCarrier | { data?: AffordanceCarrier[] } | undefined,
  requestPath: string,
): void {
  if (!import.meta.env.DEV) return
  if (response === undefined) return

  // List responses carry `_actions` at top-level (for collection
  // verbs like create); per-item `_actions` live inside `data[i]`.
  // The whitelist callers know which shape they're handing us, so
  // we just check the top-level field. Missing per-item `_actions`
  // surfaces via the same per-request warning on the wrapping list.
  if ((response as AffordanceCarrier)._actions !== undefined) return

  // Also accept the {data, _actions} list shape — if the list root
  // carries _actions we're fine even if `response` doesn't.
  const asList = response as { _actions?: Record<string, boolean>; data?: unknown }
  if (asList._actions !== undefined) return

  if (seen.has(requestPath)) return
  seen.add(requestPath)
  // eslint-disable-next-line no-console
  console.warn(
    `[affordances] Response from ${requestPath} is missing the _actions field. ` +
      `The data-entry server should always emit it; this likely indicates a ` +
      `server-side regression. The UI will render write controls defensively; ` +
      `the server still enforces ACLs on the write.`,
  )
}

// Clear the dedup memory on HMR so a fresh module instance can warn
// again. Without this, an HMR-reloaded API client keeps the stale Set
// from the previous instance and the warning gets suppressed for the
// rest of the session.
if (import.meta.hot) {
  import.meta.hot.dispose(() => {
    seen.clear()
  })
}

// Test-only reset hook. Tests that exercise the warning across
// multiple cases need a way to clear the dedup between cases without
// touching the production code path.
export function _resetAffordancesWarningForTests(): void {
  seen.clear()
}
