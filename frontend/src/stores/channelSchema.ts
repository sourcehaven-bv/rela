import { defineStore } from 'pinia'
import { ref, shallowRef } from 'vue'
import { getChannelSchema } from '@/api/channels'
import { useEvents } from '@/composables/useEvents'
import { runWhenVisible } from '@/composables/visibleRefetch'
import { useSchemaStore } from '@/stores/schema'
import {
  MESSAGE_OPS,
  type ChannelHead,
  type ChannelMessageTypeSchema,
  type ChannelTypeSchema,
  type Message,
} from '@/types/channels'

/**
 * The channel type descriptor, `GET /_schema/channels` (TKT-91BK6F): the
 * message types, properties and labels realtime.yaml declares.
 *
 * It is not part of the schema store's boot load. It loads the first time
 * a page needs it, and again on `refresh`, which the server sends after a
 * realtime.yaml reload.
 *
 * A failed load keeps what is held: before the first success that is an
 * empty list, so channel surfaces stay hidden while the rest of the app
 * works; after it, the descriptor that worked, so one failed reload does
 * not take open channels away. `loaded` stays false until a load succeeds,
 * and ensureLoaded tries again once RETRY_BASE_MS has passed, doubling up
 * to RETRY_MAX_MS after each failure.
 */
export const RETRY_BASE_MS = 5_000
export const RETRY_MAX_MS = 60_000

export const useChannelSchemaStore = defineStore('channelSchema', () => {
  const channelTypes = shallowRef<ChannelTypeSchema[]>([])
  /** Whether a load has succeeded; channelTypes is then a real answer. */
  const loaded = ref(false)
  let failures = 0
  // Before this time ensureLoaded does not try a failed first load again.
  let retryAt = 0
  let inflight: Promise<void> | null = null
  // Bumped by reload, so the answer of a superseded load does not land.
  let generation = 0
  // Names a reload was already asked for, so a head that keeps naming an
  // unknown type does not reload on every frame.
  const reloadedFor = new Set<string>()

  async function fetchSchema(gen: number) {
    try {
      const types = (await getChannelSchema()).channel_types ?? []
      if (gen !== generation) return
      channelTypes.value = types
      loaded.value = true
      failures = 0
    } catch {
      if (gen !== generation) return
      // Keep what is held: hidden before the first load, unchanged after.
      failures++
      retryAt = Date.now() + Math.min(RETRY_BASE_MS * 2 ** (failures - 1), RETRY_MAX_MS)
    }
  }

  /**
   * Loads once; concurrent callers share the request. After a failed first
   * load it tries again only once the retry delay has passed.
   */
  function ensureLoaded(): Promise<void> {
    if (loaded.value) return Promise.resolve()
    if (!inflight && Date.now() < retryAt) return Promise.resolve()
    inflight ??= fetchSchema(generation).finally(() => {
      inflight = null
    })
    return inflight
  }

  /** Loads again, replacing what is held when the answer arrives. */
  function reload(): Promise<void> {
    const gen = ++generation
    inflight = fetchSchema(gen).finally(() => {
      if (gen === generation) inflight = null
    })
    return inflight
  }

  /** The channel types an entity type carries. */
  function channelTypesFor(entityType: string): ChannelTypeSchema[] {
    return channelTypes.value.filter((ct) => ct.attach_to.includes(entityType))
  }

  function channelType(name: string): ChannelTypeSchema | undefined {
    return channelTypes.value.find((ct) => ct.name === name)
  }

  function messageType(
    channelTypeName: string,
    name: string
  ): ChannelMessageTypeSchema | undefined {
    return channelType(channelTypeName)?.message_types.find((mt) => mt.name === name)
  }

  /**
   * Reloads once when a head or page names a message type or action the
   * held descriptor lacks: realtime.yaml changed, and the `refresh` that
   * said so may not have arrived yet.
   */
  function checkNames(channelTypeName: string, head: ChannelHead | null, messages: Message[]) {
    if (!loaded.value) return
    const unknown: string[] = []
    const known = (mt: string) => messageType(channelTypeName, mt) !== undefined
    for (const mt of head?._actions.post ?? []) {
      if (!known(mt)) unknown.push(`type:${mt}`)
    }
    for (const m of messages) {
      if (!m.type || m.removed) continue
      const mt = messageType(channelTypeName, m.type)
      if (!mt) {
        unknown.push(`type:${m.type}`)
        continue
      }
      for (const a of m._actions ?? []) {
        if (!MESSAGE_OPS.has(a) && !mt.actions.some((x) => x.name === a)) {
          unknown.push(`action:${m.type}.${a}`)
        }
      }
    }
    const fresh = unknown.map((n) => `${channelTypeName}/${n}`).filter((n) => !reloadedFor.has(n))
    if (fresh.length === 0) return
    for (const n of fresh) reloadedFor.add(n)
    void reload()
  }

  return {
    channelTypes,
    loaded,
    ensureLoaded,
    reload,
    channelTypesFor,
    channelType,
    messageType,
    checkNames,
  }
})

/**
 * The channel schema store for a component: loads it when needed and
 * reloads it on `refresh`, once the tab is visible, together with the
 * `channels:` presentation the schema store holds, so a page never mixes a
 * new descriptor with an old presentation.
 */
export function useChannelSchema() {
  const store = useChannelSchemaStore()
  const schema = useSchemaStore()
  void store.ensureLoaded()
  const { on } = useEvents()
  on('refresh', () =>
    runWhenVisible('channel-schema', () => {
      void store.reload()
      void schema.reloadChannelPresentations()
    })
  )
  return store
}
