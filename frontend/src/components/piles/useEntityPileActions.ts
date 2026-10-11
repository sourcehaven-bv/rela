import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'
import { usePiles } from '@/composables/usePiles'
import { knownPileName } from '@/composables/pileNames'
import type { EntityAction } from '@/components/entity/entityActions'
import { pileIcon } from './pileIcon'

/**
 * The pile entries of an entity page's action list (TKT-K3RJLH): add the row
 * on screen to each pile, start a new pile with it, and, while stepping
 * through a pile, remove it from that pile.
 *
 * `address` is the page's served address (`ID@face`), never the route id: a
 * pile holds the face on screen. `onRemoved` runs after the row left the
 * pile being stepped through, so the page can move on to a neighbour.
 */
export function useEntityPileActions(address: () => string | null, onRemoved: () => void) {
  const route = useRoute()
  const piles = usePiles()
  const newPileOpen = ref(false)

  /** The pile this page is being stepped through, from `?from=pile&pile=<id>`. */
  const scopePileId = computed(() => {
    if (route.query.from !== 'pile') return null
    const id = route.query.pile
    return typeof id === 'string' && id ? id : null
  })

  async function removeFromScopePile() {
    const id = scopePileId.value
    const addr = address()
    if (!id || !addr) return
    const listed = piles.piles.value.find((p) => p.id === id)
    const pile = listed ?? { id, name: knownPileName(id) ?? 'the pile' }
    if (await piles.removeItems(pile, [addr])) onRemoved()
  }

  const actions = computed<EntityAction[]>(() => {
    const addr = address()
    if (!piles.available.value || addr === null) return []
    const out: EntityAction[] = piles.piles.value.map((p) => ({
      id: `pile:${p.id}`,
      label: `Add to ${p.name}`,
      group: 'manage',
      icon: pileIcon(p.icon),
      run: () => void piles.addItems(p, [addr]),
    }))
    out.push({
      id: 'pile:new',
      label: 'Add to new pile…',
      group: 'manage',
      icon: 'plus',
      run: () => (newPileOpen.value = true),
    })
    if (scopePileId.value) {
      out.push({
        id: 'pile:remove',
        label: 'Remove from pile',
        group: 'manage',
        icon: 'remove',
        run: () => void removeFromScopePile(),
      })
    }
    return out
  })

  return {
    available: piles.available,
    actions,
    newPileOpen,
    scopePileId,
    removeFromScopePile,
  }
}
