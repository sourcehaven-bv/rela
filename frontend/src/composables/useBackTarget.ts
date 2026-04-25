import { computed, type ComputedRef } from 'vue'
import { useRoute } from 'vue-router'
import { readReturnTo } from '@/utils/returnPath'

/**
 * LabelHint — structured hint for the caller to render a human-friendly
 * Back button label. Deliberately keeps label *resolution* out of this
 * composable so we don't couple a generic navigation helper to the
 * data-entry schemaStore (see TKT-JIEKC RR-RV4LA).
 */
export type LabelHint = { kind: 'list'; id: string }

export interface BackTarget {
  /** Validated same-origin path to push via vue-router. */
  to: string
  /** Render hint; `null` when the caller should use the default "← Back". */
  labelHint: LabelHint | null
}

/**
 * useBackTarget is the single source of truth for "does this view have
 * somewhere to go back to, and where?"
 *
 * Precedence:
 *   1. `?return_to=<safe-path>` — used verbatim, no label hint (caller
 *      renders generic "← Back"). This is the mechanism the document link
 *      rewriter (see TKT-4MFUK) emits on all internal doc links.
 *   2. `?from=<list-id>` — used when the user is in a list-scoped context
 *      (EntityView and CustomView thread `?from=` through their scope
 *      navigation). Label hint carries the list id so the caller can
 *      resolve its title.
 *   3. Neither — returns `null`. Callers gate the Back button on this.
 *
 * Returned as a reactive `computed` because `route.query` can mutate via
 * `router.replace` (e.g. DocumentsPanel writing `?doc=X`) and the Back
 * button must reflect that without a remount.
 *
 * Unsafe `return_to` values (open-redirect payloads) are treated as
 * absent — the composable falls through to `?from=` or null. The same
 * `isSafeReturnPath` guard runs on the server (see
 * internal/dataentry/return_path.go) before the value ever reaches the
 * client.
 */
export function useBackTarget(): ComputedRef<BackTarget | null> {
  const route = useRoute()
  return computed<BackTarget | null>(() => {
    const safe = readReturnTo(route.query)
    if (safe) return { to: safe, labelHint: null }
    const from = typeof route.query.from === 'string' ? route.query.from : null
    if (from) return { to: `/list/${from}`, labelHint: { kind: 'list', id: from } }
    return null
  })
}
