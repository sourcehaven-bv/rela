import { useConfigureSnapshot } from '@/queries/configure'
import { useConfigDraftStore } from '@/stores/configDraft'

/**
 * Drops every unsaved change and fetches the configuration again, so a new
 * draft starts from the version the server holds now rather than the one
 * the old draft was made against. Every "discard" control uses this.
 */
export function useDiscardDraft(): () => Promise<void> {
  const draft = useConfigDraftStore()
  const { refetch } = useConfigureSnapshot()
  return async () => {
    draft.discardAll()
    await refetch()
  }
}
