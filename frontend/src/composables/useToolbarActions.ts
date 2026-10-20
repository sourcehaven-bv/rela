import { computed, ref, toValue, type MaybeRefOrGetter } from 'vue'
import { useRouter } from 'vue-router'
import { useSchemaStore } from '@/stores'
import { runAction } from '@/api/actions'
import { useActionFeedback } from './useActionFeedback'
import { useConfirm } from './useConfirm'
import type { ActionConfig } from '@/types'

/** One toolbar button: the action id, its config and the button text. */
export interface ToolbarAction {
  id: string
  label: string
  config: ActionConfig
}

/**
 * The buttons a list's `toolbar_actions:` puts in its page header, and the
 * click handler that runs one.
 *
 * A button shows only when the list response carries
 * `_actions['action:<id>'] === true`. The server emits that key for an
 * action whose `permission:` the user holds; absence means "not offered",
 * the detail-page action rule, so the SPA evaluates nothing itself.
 *
 * A run sends no entity, like a sidebar action, and reacts to the result the
 * way the sidebar does: follow a `redirect`, toast a `message`. Without a
 * redirect `onDone` runs, so the list can refetch what the script created.
 */
export function useToolbarActions(
  listId: MaybeRefOrGetter<string>,
  collectionActions: MaybeRefOrGetter<Record<string, boolean> | undefined>,
  onDone: () => void
) {
  const schemaStore = useSchemaStore()
  const router = useRouter()
  const { confirm } = useConfirm()
  const { reportResult, reportError } = useActionFeedback()
  const busy = ref(false)

  const buttons = computed<ToolbarAction[]>(() => {
    const ids = schemaStore.getList(toValue(listId))?.toolbar_actions ?? []
    const offered = toValue(collectionActions) ?? {}
    const out: ToolbarAction[] = []
    for (const id of ids) {
      const config = schemaStore.getAction(id)
      if (!config || offered[`action:${id}`] !== true) continue
      out.push({ id, label: config.label || id, config })
    }
    return out
  })

  async function run(action: ToolbarAction, ev?: Event) {
    if (busy.value) return
    const { id, label, config } = action
    const triggerEl = ev?.currentTarget instanceof HTMLElement ? ev.currentTarget : null
    if (config.confirm) {
      const ok = await confirm({
        title: `${label}?`,
        message: typeof config.confirm === 'string' ? config.confirm : `Run ${label}?`,
        confirmLabel: label,
      })
      if (!ok) return
    }
    busy.value = true
    try {
      const res = await runAction(id)
      reportResult(res, `${label}: done`)
      if (res?.redirect) {
        void router.push(res.redirect)
        return
      }
      onDone()
    } catch (err) {
      reportError(err, triggerEl, label)
    } finally {
      busy.value = false
    }
  }

  return { buttons, busy, run }
}
