/**
 * The Lua actions an entity page offers (TKT-VVS16W), in the order it draws
 * them.
 *
 * The server publishes each offered action as `_actions['action:<id>']: true`
 * and decides type, face, `when` and `permission` itself; this only joins the
 * keys with their config and sorts them.
 */
import type { ActionConfig } from '@/types'

export interface DetailAction {
  id: string
  label: string
  config: ActionConfig
}

const ACTION_KEY_PREFIX = 'action:'

/**
 * The actions offered by `affordances`, sorted by `order:` (absent is 0),
 * then by label, then by id. A key whose action the config does not declare
 * is left out: there is nothing to label or run.
 */
export function offeredDetailActions(
  affordances: Record<string, boolean> | undefined,
  lookup: (id: string) => ActionConfig | undefined
): DetailAction[] {
  const out: DetailAction[] = []
  for (const [key, allowed] of Object.entries(affordances ?? {})) {
    if (!allowed || !key.startsWith(ACTION_KEY_PREFIX)) continue
    const id = key.slice(ACTION_KEY_PREFIX.length)
    const config = lookup(id)
    if (config) out.push({ id, label: config.label || id, config })
  }
  return out.sort(
    (a, b) =>
      (a.config.order ?? 0) - (b.config.order ?? 0) ||
      a.label.localeCompare(b.label) ||
      a.id.localeCompare(b.id)
  )
}
