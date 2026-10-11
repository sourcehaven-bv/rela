import { useQuery } from '@pinia/colada'
import { getMe } from '@/api'

/**
 * The current principal (`GET /_me`). Shared by the account menu and the
 * Configure entry, so the request is made once.
 *
 * Identity changes only with a new login, which is a page load, so the
 * answer never goes stale.
 */
export function useMe() {
  return useQuery({
    key: ['me'],
    query: getMe,
    staleTime: Infinity,
  })
}
