/**
 * The selected WORLD, synced to the `?world=` URL query param.
 *
 * A world selects which FACE of each entity is served — and which entities
 * appear at all, since an entity with no face in a world is omitted entirely
 * (existence in a world IS the publication bit). See the design notes in
 * `internal/dataentry/world.go`.
 *
 * ## Why the URL, and why a query param
 *
 * The world belongs in the URL so a world-bound view is a shareable link: "the
 * published policies" has to survive a copy-paste, a bookmark and a reload, or
 * a reviewer cannot be pointed at what a reader actually sees. It is a QUERY
 * param rather than a path segment because it scopes an existing view rather
 * than naming a different one — the same list, seen through a different world
 * — which is also how the API spells it (`?world=`).
 *
 * ## Why this is separate from useUrlFilterSync
 *
 * A world is not a filter. `useUrlFilterSync` owns `filter[...]` + `q`, writes
 * with `router.replace` (no history entry per keystroke, right for typing) and
 * echo-guards on a filter-shaped signature. Switching world is a deliberate,
 * infrequent act that SHOULD be undoable with the back button, so it uses
 * `router.push`. Folding it into the filter signature would also make every
 * world change look like a filter echo. Same URL, different lifecycle.
 *
 * ## The empty string is the default world
 *
 * `''` means "no `?world=`", which the API reads as the default world. The API
 * also accepts the explicit spelling `?world=default`; both normalize to the
 * default world in the entity cache (see `worldKey` in `stores/entities.ts`),
 * but this composable keeps whatever the URL said so a deep link round-trips
 * unchanged rather than being silently rewritten.
 */
import { computed, type Ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

// DEFAULT_WORLD is the reserved name the API accepts as an explicit way to
// spell the implicit default world (`defaultWorldName`, dataentry/world.go).
export const DEFAULT_WORLD = 'default'

function readWorldParam(value: unknown): string {
  // An array means the URL carried `?world=` twice. The API REJECTS that
  // outright (400 duplicate_world) rather than picking one by a precedence
  // rule nobody would remember, so the SPA must not invent a precedence
  // either — taking the last would send a single value the user never asked
  // for and hide the malformed link. Resolve to the default world; the
  // selector then shows the default world, which is what gets served.
  if (Array.isArray(value)) return ''
  return typeof value === 'string' ? value : ''
}

export interface UseWorld {
  /** The world named by the URL; `''` for the default world. */
  world: Readonly<Ref<string>>
  /** True when a non-default world is active. */
  isWorldBound: Readonly<Ref<boolean>>
  /**
   * The value to send as the API's `world` param: `undefined` for the default
   * world, so callers can spread it into a params object without emitting an
   * empty `?world=`.
   */
  worldParam: Readonly<Ref<string | undefined>>
  /** Select a world. `''` (or DEFAULT_WORLD) returns to the default world. */
  setWorld: (next: string) => void
}

export function useWorld(): UseWorld {
  const route = useRoute()
  const router = useRouter()

  const world = computed(() => readWorldParam(route.query.world))

  const isWorldBound = computed(
    () => world.value !== '' && world.value !== DEFAULT_WORLD,
  )

  const worldParam = computed(() =>
    isWorldBound.value ? world.value : undefined,
  )

  function setWorld(next: string) {
    if (next === world.value) return
    const query = { ...route.query }
    if (next === '' || next === DEFAULT_WORLD) {
      delete query.world
    } else {
      query.world = next
    }
    // Changing world resets pagination: page 3 of the draft world is not
    // page 3 of the published world — the published world may hold fewer
    // entities than that page's offset, landing the user on a silently empty
    // page that reads as "nothing is published".
    delete query.page
    // push, not replace: switching world is a deliberate act the back button
    // should undo. See the composable doc.
    router.push({ query })
  }

  return { world, isWorldBound, worldParam, setWorld }
}
