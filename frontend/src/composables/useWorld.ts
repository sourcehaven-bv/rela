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
import { useSchemaStore } from '@/stores/schema'

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
   * The value to send as the API's `world` param. `undefined` for the default
   * world, so callers can spread it into a params object without emitting an
   * empty `?world=` — unless the operator configured `default_world`, in which
   * case an ABSENT param means the configured world to the server and the only
   * spelling of "the bare faces" is an explicit `default`.
   */
  worldParam: Readonly<Ref<string | undefined>>
  /** Select a world. `''` (or DEFAULT_WORLD) returns to the default world. */
  setWorld: (next: string) => void
}

export function useWorld(): UseWorld {
  // Both are undefined when this runs outside a router context. That is a real
  // case, not a test artifact: RelationPicker reads the world so its candidate
  // query is world-scoped, and it is mounted inside forms and modals that are
  // unit-tested (and could be embedded) without a router.
  //
  // Degrading to "no URL world" is the right failure here. The world is then
  // the operator's configured default, which is exactly what a bare URL means,
  // and setWorld becomes a no-op rather than throwing — a component that cannot
  // navigate should not be able to half-navigate. The alternative, letting the
  // injection error escape, took down the whole candidate load and left the
  // picker empty, which is a much worse answer than "the default world".
  const route = useRoute() as ReturnType<typeof useRoute> | undefined
  const router = useRouter() as ReturnType<typeof useRouter> | undefined

  const schemaStore = useSchemaStore()

  // An absent `?world=` means the OPERATOR'S default world, not the raw
  // default face. For an ISMS that is the whole point: browsing shows what is
  // in force, and a draft is reached by naming an editorial world.
  //
  // An explicit `?world=` always wins, including `?world=default`, which is
  // how a reader gets to the raw faces when a default is configured.
  //
  // This is presentation only. The world's read grant is re-checked per
  // request exactly as for an explicit param, so a configured default can
  // change which face a bare URL resolves to and nothing else. If the caller
  // may not read it, they get that world's ordinary denial.
  //
  // The SERVER applies the same default (`attachWorld`/`resolveWorld` in
  // internal/dataentry/world.go), so this is no longer the only thing
  // enforcing it — a bare `curl` now lands in the same world the browser
  // does. It used to be SPA-only, and that WAS the defect: a face-scoped
  // role read nothing from the raw API despite holding a valid grant.
  //
  // Kept because it still drives UI STATE, not just the request: the world
  // selector, the "Go to draft" affordances, and `setWorld`'s decision about
  // when to drop the query param all read `world`. The resulting explicit
  // `?world=` on requests agrees with what the server would have defaulted
  // to, so the two cannot disagree.
  const world = computed(() => {
    const explicit = route?.query?.world
    if (explicit !== undefined) return readWorldParam(explicit)
    return schemaStore.defaultWorld || ''
  })

  const isWorldBound = computed(
    () => world.value !== '' && world.value !== DEFAULT_WORLD,
  )

  // `undefined` for the default world so a params spread emits no `?world=` —
  // EXCEPT when a default is configured. The server applies `default_world`
  // to a request with NO param, so on such a deployment dropping the param
  // does not mean "the bare faces", it means "the configured world". This
  // used to drop it: "Go to draft" wrote `?world=default` into the URL, the
  // page treated itself as the writable default world, and the request
  // fetched the PUBLISHED face — every write guard off over published bytes.
  const worldParam = computed(() => {
    if (isWorldBound.value) return world.value
    return schemaStore.defaultWorld ? DEFAULT_WORLD : undefined
  })

  function setWorld(next: string) {
    if (next === world.value) return
    if (!route || !router) return
    const query = { ...route.query }
    // Dropping the param lands on the operator's default world, so it is only
    // the way to reach `next` when `next` IS that default. Otherwise the param
    // has to be written explicitly — including `?world=default`, which is how
    // a reader reaches the raw faces on a deployment that configures one.
    // '' and DEFAULT_WORLD name the SAME world, so normalise before comparing
    // — otherwise setWorld(DEFAULT_WORLD) on a deployment with no configured
    // default writes ?world=default instead of dropping the param.
    const norm = (w: string) => (w === DEFAULT_WORLD ? '' : w)
    if (norm(next) === norm(schemaStore.defaultWorld)) {
      delete query.world
    } else if (norm(next) === '') {
      query.world = DEFAULT_WORLD
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
