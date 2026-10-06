import { useQuery } from '@pinia/colada'
import { getConfigure } from '@/api/configure'

export const configureKeys = {
  snapshot: ['configure'] as const,
}

/**
 * The configuration as the server holds it now. The Configure space edits a
 * draft on top of it.
 *
 * Never refetched on its own: the draft is based on one version, and a new
 * version arriving under it would change what the draft's changes mean. A
 * save reloads the page, and "Discard all" refetches explicitly.
 */
export function useConfigureSnapshot() {
  return useQuery({
    key: configureKeys.snapshot,
    query: getConfigure,
    staleTime: Infinity,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
  })
}
