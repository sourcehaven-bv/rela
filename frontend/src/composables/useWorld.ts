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
 * `''` means "no `?world=`", which the API reads as the default world:
 * `default_world:`, else the first declared world, else the generated
 * `default` world when the schema declares none. `?world=default` is valid
 * only in that last case; a schema with declared worlds answers it with a 400,
 * and App.vue drops it from the URL (see unknownWorldQuery).
 */
import { computed, type Ref } from 'vue'
import { useRoute, useRouter, type LocationQuery, type LocationQueryRaw } from 'vue-router'
import { useSchemaStore } from '@/stores/schema'

// DEFAULT_WORLD is the name of the generated default world, which exists only
// when the schema declares no worlds. A schema that declares worlds has no
// world by this name, and the API answers `?world=default` with a 400.
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
   * The value to send as the API's `world` param. `undefined` for the
   * generated default world, so callers can spread it into a params object
   * without emitting an empty `?world=`.
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
  // An explicit `?world=` always wins.
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

  // `undefined` for the generated default world so a params spread emits no
  // `?world=`. A page is unbound only when the schema declares no worlds, so
  // there is no other world an absent param could mean.
  const worldParam = computed(() => (isWorldBound.value ? world.value : undefined))

  function setWorld(next: string) {
    if (next === world.value) return
    if (!route || !router) return
    // push, not replace: switching world is a deliberate act the back button
    // should undo. See the composable doc.
    router.push({ query: worldQuery(next, route.query, schemaStore.defaultWorld) })
  }

  return { world, isWorldBound, worldParam, setWorld }
}

/**
 * Spells a world into a query, the ONE place that rule lives. `setWorld`
 * uses it for a same-page switch; a caller that also changes the path (a
 * copy landing in another world) uses it directly, so the two cannot
 * drift — the first copy of this logic had already lost the page reset.
 *
 * Dropping the param lands on the default world, so it is only the way to
 * reach `next` when `next` IS that default. Otherwise the param is written
 * explicitly. '' and DEFAULT_WORLD name the same world on a schema that
 * declares none, so both are normalised before comparing; otherwise
 * DEFAULT_WORLD would write ?world=default instead of dropping the param.
 *
 * Changing world resets pagination: page 3 of the draft world is not page 3
 * of the published world — the published world may hold fewer entities than
 * that page's offset, landing the user on a silently empty page that reads
 * as "nothing is published".
 */
export function worldQuery(next: string, base: LocationQuery, defaultWorld: string): LocationQueryRaw {
  const query: LocationQueryRaw = { ...base }
  const norm = (w: string) => (w === DEFAULT_WORLD ? '' : w)
  if (norm(next) === '' || norm(next) === norm(defaultWorld)) {
    delete query.world
  } else {
    query.world = next
  }
  delete query.page
  return query
}

/**
 * The query to replace the URL with when its `?world=` names no world this
 * server serves, or null when the URL is fine.
 *
 * The API answers such a request with a 400 on every route, so a stale
 * bookmark (`?world=default` on a schema that now declares worlds) would
 * break every page. Dropping the parameter lands the user in the default
 * world, which is what a bare URL means. A repeated `?world=` is dropped
 * for the same reason. Returns null until the schema has loaded, because an
 * empty world map cannot tell a stale name from an unloaded one.
 */
export function unknownWorldQuery(
  query: LocationQuery,
  worlds: ReadonlyMap<string, unknown>,
): LocationQueryRaw | null {
  const value = query.world
  if (value === undefined || worlds.size === 0) return null
  if (typeof value === 'string' && (value === '' || worlds.has(value))) return null
  const next: LocationQueryRaw = { ...query }
  delete next.world
  delete next.page
  return next
}
