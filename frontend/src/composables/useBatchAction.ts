import { ref } from 'vue'
import { runBatchAction, type ActionBatch } from '@/api/actions'
import { getErrorMessage } from '@/api/errors'
import { useConfirm } from '@/composables/useConfirm'
import { useUIStore } from '@/stores'
import { t } from '@/i18n'
import type { ActionConfig } from '@/types'

/** Whether an action runs on several entities in one request (TKT-I2SRNWR). */
export function isBatchAction(action: ActionConfig | undefined): boolean {
  return !!action?.llm?.batch
}

/** A finished batch run, for the review dialog. */
export interface BatchResult {
  batch: ActionBatch
  label: string
  entityType: string
}

/**
 * Runs a batch `llm:` action over the entities on screen (TKT-I2SRNWR). It
 * refuses more entities than the action's `batch.max`, asks before sending
 * (the entities' data goes to an AI provider; the action's `confirm:` text
 * replaces the generic question), and keeps the reply in
 * `result` for the review dialog. The run writes nothing.
 */
export function useBatchAction() {
  const { confirm } = useConfirm()
  const uiStore = useUIStore()
  const pending = ref(false)
  const result = ref<BatchResult | null>(null)
  // Set from the first click until the run ends, confirm included, so a
  // second click or key press while the confirm is open starts nothing.
  let starting = false

  /**
   * Starts a run over addresses. tooMany is the message shown when there
   * are more than the action allows; it names what the user can do on the
   * surface that started the run.
   */
  async function start(
    actionId: string,
    action: ActionConfig,
    entityType: string,
    addresses: string[],
    tooMany: (n: number, max: number) => string
  ): Promise<void> {
    if (starting || addresses.length === 0) return
    const label = action.label || actionId
    const max = action.llm?.batch?.max ?? 0
    if (addresses.length > max) {
      uiStore.error(tooMany(addresses.length, max))
      return
    }
    starting = true
    try {
      // The operator's confirm text when there is one; the generic one says
      // that the data goes to the AI provider.
      const ok = await confirm({
        title: `${label}?`,
        message:
          typeof action.confirm === 'string' && action.confirm
            ? action.confirm
            : t('entity.llmBatch.confirm', { n: addresses.length }, addresses.length),
        confirmLabel: label,
      })
      if (!ok) return
      pending.value = true
      result.value = { batch: await runBatchAction(actionId, addresses), label, entityType }
    } catch (err) {
      uiStore.error(getErrorMessage(err, t('entity.llmBatch.runFailed', { label })))
    } finally {
      pending.value = false
      starting = false
    }
  }

  function close() {
    result.value = null
  }

  return { pending, result, start, close }
}
