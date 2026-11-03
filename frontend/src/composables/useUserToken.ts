import { computed } from 'vue'
import { useMe } from '@/queries/me'
import { USER_TOKEN } from '@/utils/userToken'

/**
 * The signed-in user's id, for resolving `$user` in values (TKT-YJSRBZ).
 * `/_me` is asked only once one of values is the token, so a form or board
 * that does not use it makes no request.
 */
export function useUserToken(values: () => unknown[]) {
  const { data } = useMe(() => values().includes(USER_TOKEN))
  return computed(() => data.value?.user)
}
