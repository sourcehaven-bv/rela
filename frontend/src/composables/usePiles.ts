import { computed, onBeforeUnmount, watch, type Ref } from 'vue'
import { useQuery, useQueryCache } from '@pinia/colada'
import {
  addPileItems,
  createPile,
  deletePile,
  getPile,
  listPiles,
  pileErrorCode,
  removePileItems,
  updatePile,
  PILE_REQUEST_MAX_ITEMS,
  type CreatePileInput,
  type Pile,
  type PileSummary,
  type UpdatePileInput,
} from '@/api/piles'
import { getErrorMessage } from '@/api/errors'
import { t } from '@/i18n'
import { useSchemaStore, useUIStore } from '@/stores'
import { useEvents } from '@/composables/useEvents'
import { useWorld } from '@/composables/useWorld'
import { useFlyout } from '@/composables/useFlyout'
import { pileNavId } from '@/components/common/sidebarNav'
import { forgetPileName, rememberPileNames, resetPileNames } from './pileNames'
import { runWhenVisible } from './visibleRefetch'

/** How long a burst of entity changes is left to settle before recounting. */
export const PILES_EVENT_DEBOUNCE_MS = 1000

/** How long the Undo toast after a removal stays up. */
export const PILE_UNDO_TOAST_MS = 10000

/** The query-key root of every piles query, list and detail alike. */
export const PILES_KEY_ROOT = 'piles'

/** What a mutation needs to know about a pile: its id, and its name for toasts. */
export type PileRef = Pick<PileSummary, 'id' | 'name'>

/*
 * One debounce for every usePiles instance. Several components use the
 * composable at once (sidebar, panel, menus), and each subscribes to the
 * live-update feed; a shared timer turns one burst of writes into one
 * invalidation rather than one per subscriber.
 */
let eventTimer: ReturnType<typeof setTimeout> | null = null

/** Test seam: reset module state between cases. */
export function resetPilesState(): void {
  resetPileNames()
  if (eventTimer) clearTimeout(eventTimer)
  eventTimer = null
}

/**
 * The sentence a toast appends when a selection was longer than one request
 * may carry and only its head was sent; empty otherwise. The server would
 * refuse the whole request, so the SPA sends the first
 * PILE_REQUEST_MAX_ITEMS and says so rather than dropping the rest silently.
 */
export function truncationNote(selected: number): string {
  if (selected <= PILE_REQUEST_MAX_ITEMS) return ''
  return t('piles.truncated', { max: PILE_REQUEST_MAX_ITEMS, selected })
}

/** message, followed by the truncation note when `selected` was cut. */
function withNote(message: string, selected: number): string {
  const note = truncationNote(selected)
  return note ? t('piles.withNote', { message, note }) : message
}

/**
 * The user-facing sentence for a failed piles request.
 *
 * `item_not_found` deliberately never says which address failed: the server
 * does not name it, because whether an entity exists is itself a secret.
 * `ambiguous_address` keeps the server's message, which names the faces.
 */
export function pileErrorMessage(err: unknown, fallback: string): string {
  switch (pileErrorCode(err)) {
    case 'pile_name_taken':
      return t('piles.nameTaken')
    case 'pile_limit':
      return t('piles.limit')
    case 'item_not_found':
      return t('piles.itemNotFound')
    case 'pile_not_found':
      return t('piles.notFound')
    case 'no_owner':
      return t('piles.noOwner')
    default:
      return getErrorMessage(err, fallback)
  }
}

/**
 * The user's piles, and the writes that change them (TKT-K3RJLH).
 *
 * The list is fetched only when the server says piles are available, keyed
 * on the world because counts are the READABLE items in that world. It is
 * refetched after this composable's own writes and after a burst of entity
 * changes settles (an item can become unreadable, or be deleted). It is NOT
 * refetched on navigation: nothing about a pile changes when the user moves
 * between pages.
 *
 * Every write reports its outcome through the toast queue and resolves to
 * null (or false) on failure, so a caller only decides what to do next.
 */
export function usePiles() {
  const schemaStore = useSchemaStore()
  const uiStore = useUIStore()
  const queryCache = useQueryCache()
  const flyout = useFlyout()
  const { worldParam } = useWorld()
  const { on, off } = useEvents()

  const available = computed(() => schemaStore.pilesAvailable)

  const query = useQuery({
    key: () => [PILES_KEY_ROOT, worldParam.value ?? ''],
    query: ({ signal }) => listPiles(worldParam.value, signal),
    enabled: () => available.value,
  })

  watch(
    () => query.data.value?.piles,
    (piles) => {
      if (piles) rememberPileNames(piles)
    },
    { immediate: true }
  )

  const piles = computed<PileSummary[]>(() =>
    available.value ? (query.data.value?.piles ?? []) : []
  )
  const icons = computed<string[]>(() => query.data.value?.icons ?? [])

  /** Refetches every pile query: the list and any open pile. */
  async function refresh(): Promise<void> {
    await queryCache.invalidateQueries({ key: [PILES_KEY_ROOT] }).catch(() => {})
  }

  function onEntityChanged() {
    if (!available.value) return
    if (eventTimer) clearTimeout(eventTimer)
    eventTimer = setTimeout(() => {
      eventTimer = null
      // A hidden tab refreshes once, when it is shown again.
      runWhenVisible(PILES_KEY_ROOT, () => void refresh())
    }, PILES_EVENT_DEBOUNCE_MS)
  }
  on('entity:changed', onEntityChanged)
  onBeforeUnmount(() => off('entity:changed', onEntityChanged))

  async function create(input: CreatePileInput): Promise<Pile | null> {
    try {
      const pile = await createPile({
        ...input,
        items: input.items?.slice(0, PILE_REQUEST_MAX_ITEMS),
      })
      rememberPileNames([pile])
      const n = input.items?.length ?? 0
      uiStore.showToast(
        'success',
        n > 0
          ? withNote(
              t('piles.createdWith', { name: pile.name, n: pile.count }, pile.count),
              n
            )
          : t('piles.created', { name: pile.name }),
        5000,
        { label: t('piles.open'), onAction: () => openPile(pile) }
      )
      return pile
    } catch (err) {
      uiStore.error(pileErrorMessage(err, t('piles.createFailed')))
      return null
    } finally {
      await refresh()
    }
  }

  async function update(pile: PileRef, input: UpdatePileInput): Promise<Pile | null> {
    try {
      const updated = await updatePile(pile.id, input)
      rememberPileNames([updated])
      return updated
    } catch (err) {
      uiStore.error(pileErrorMessage(err, t('piles.changeFailed')))
      return null
    } finally {
      await refresh()
    }
  }

  async function remove(pile: PileRef): Promise<boolean> {
    try {
      await deletePile(pile.id)
      forgetPileName(pile.id)
      if (flyout.pile.value?.pileId === pile.id) flyout.close()
      uiStore.success(t('piles.deleted', { name: pile.name }))
      return true
    } catch (err) {
      uiStore.error(pileErrorMessage(err, t('piles.deleteFailed')))
      return false
    } finally {
      await refresh()
    }
  }

  /**
   * Adds addresses to a pile. Resolves to how many were new, or null on
   * failure. `quiet` skips the success toast, for an Undo that already
   * announced itself.
   */
  async function addItems(
    pile: PileRef,
    addresses: string[],
    opts: { quiet?: boolean } = {}
  ): Promise<number | null> {
    if (addresses.length === 0) return 0
    const items = addresses.slice(0, PILE_REQUEST_MAX_ITEMS)
    try {
      const { added } = await addPileItems(pile.id, items)
      if (!opts.quiet) {
        const skipped = items.length - added
        const addedText =
          skipped > 0
            ? t('piles.addedSkipped', { n: added, name: pile.name, skipped })
            : t('piles.added', { n: added, name: pile.name })
        const message = withNote(addedText, addresses.length)
        uiStore.showToast('success', message, 5000, {
          label: t('piles.open'),
          onAction: () => openPile(pile),
        })
      }
      return added
    } catch (err) {
      uiStore.error(pileErrorMessage(err, t('piles.addFailed', { name: pile.name })))
      return null
    } finally {
      await refresh()
    }
  }

  /** Removes addresses from a pile, offering Undo, which adds them back as the newest. */
  async function removeItems(pile: PileRef, addresses: string[]): Promise<boolean> {
    if (addresses.length === 0) return true
    try {
      await removePileItems(pile.id, addresses)
      uiStore.showToast(
        'success',
        t('piles.removed', { n: addresses.length, name: pile.name }, addresses.length),
        PILE_UNDO_TOAST_MS,
        {
          label: t('common.undo'),
          onAction: () => {
            void addItems(pile, addresses, { quiet: true }).then((added) => {
              if (added !== null)
                uiStore.success(
                  t('piles.restored', { n: addresses.length, name: pile.name }, addresses.length)
                )
            })
          },
        }
      )
      return true
    } catch (err) {
      uiStore.error(pileErrorMessage(err, t('piles.removeFailed', { name: pile.name })))
      return false
    } finally {
      await refresh()
    }
  }

  function openPile(pile: PileRef) {
    flyout.openPile({ navId: pileNavId(pile.id), title: pile.name, pileId: pile.id })
  }

  return {
    available,
    piles,
    icons,
    isPending: computed(() => query.isPending.value),
    refresh,
    create,
    update,
    remove,
    addItems,
    removeItems,
    openPile,
  }
}

/**
 * One pile with its items, for the panel. Shares the root key with the list,
 * so the list's invalidations refetch it too.
 */
export function usePile(id: Ref<string | null>) {
  const schemaStore = useSchemaStore()
  const { worldParam } = useWorld()
  return useQuery({
    key: () => [PILES_KEY_ROOT, worldParam.value ?? '', 'pile', id.value ?? ''],
    query: ({ signal }) => getPile(id.value ?? '', worldParam.value, signal),
    enabled: () => schemaStore.pilesAvailable && !!id.value,
  })
}
