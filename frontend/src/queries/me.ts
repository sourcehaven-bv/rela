import { useQuery } from '@pinia/colada'
import { getMe } from '@/api'
import { USER_TOKEN } from '@/utils/userToken'

/**
 * The current principal (`GET /_me`). Shared by the account menu and the
 * Configure entry, so the request is made once.
 *
 * Identity changes only with a new login, which is a page load, so the
 * answer never goes stale.
 */
export function useMe(enabled: () => boolean = () => true) {
  return useQuery({
    key: ['me'],
    query: getMe,
    staleTime: Infinity,
    enabled,
  })
}

/**
 * useMe for a component that needs the principal only to resolve `$user` in
 * values: the request is made only once one of them is the token.
 */
export function useMeFor(values: () => unknown[]) {
  return useMe(() => values().includes(USER_TOKEN))
}
