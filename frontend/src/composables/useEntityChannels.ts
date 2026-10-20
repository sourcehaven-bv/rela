import { onScopeDispose, shallowRef, watch, type MaybeRefOrGetter, toValue } from 'vue'
import { listChannels, type ChannelAnchor } from '@/api/channels'
import { liveChannelsPath } from '@/api/live'
import { shouldDropHeldContent } from '@/api/errors'
import { useLiveSubscription } from '@/composables/useLiveSubscription'
import type { ChannelState, ChannelTypeList } from '@/types/channels'

export interface UseEntityChannelsOptions {
  /** Lists only channels in this state. */
  state?: MaybeRefOrGetter<ChannelState | undefined>
  /**
   * While false the list keeps what it holds but is not live: the panel
   * drops the subscription while one channel is open, to stay within the
   * stream's subscription budget. It refetches when it turns true again.
   */
  live?: MaybeRefOrGetter<boolean>
}

/**
 * The channels of one entity, grouped by channel type, live on the
 * `_channels` path (TKT-91BK6F).
 *
 * A refused refetch (401/403/404) clears the list: holding it would keep
 * showing channels on an entity this user may no longer read. Any other
 * failure keeps it.
 *
 * `anchor` null means nothing to show yet.
 */
export function useEntityChannels(
  anchor: MaybeRefOrGetter<ChannelAnchor | null>,
  options: UseEntityChannelsOptions = {}
) {
  const types = shallowRef<ChannelTypeList[]>([])
  const error = shallowRef<unknown>(null)
  let seq = 0

  const isLive = () => toValue(options.live) ?? true

  async function refetch() {
    const a = toValue(anchor)
    const mine = ++seq
    if (!a) {
      types.value = []
      return
    }
    try {
      const res = await listChannels(a, toValue(options.state))
      if (mine !== seq) return
      types.value = res.types
      error.value = null
    } catch (err) {
      if (mine !== seq) return
      error.value = err
      if (shouldDropHeldContent(err)) types.value = []
    }
  }

  const key = () => {
    const a = toValue(anchor)
    return a ? [a.type, a.address, a.world ?? '', toValue(options.state) ?? ''].join('\u0000') : ''
  }
  watch(
    key,
    () => {
      // Another entity's channels must not linger while the new list loads.
      types.value = []
      error.value = null
      void refetch()
    },
    { immediate: true }
  )
  watch(isLive, (live, was) => {
    if (live && !was) void refetch()
  })

  // An answer that arrives after the component is gone must not land.
  onScopeDispose(() => {
    seq++
  })

  useLiveSubscription(
    'entity-channels',
    () => {
      const a = toValue(anchor)
      return a && isLive() ? liveChannelsPath(a, toValue(options.state)) : null
    },
    () => void refetch()
  )

  return { types, error, refetch }
}
