import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { relaBase } from '@/api/base'
import type { SidebarCreate, SidebarSpace } from '@/types'

/**
 * Matches a space prefix: `/s/<id>` followed by the rest of the path.
 *
 * The id pattern is the one the server validates `spaces[].id` against, so a
 * path that matches here names a space the config could declare.
 */
const SPACE_PATH = /^\/s\/([a-z][a-z0-9_-]{0,31})(\/.*)?$/

/** The space id a path is prefixed with, or undefined for an unprefixed path. */
export function spaceOf(path: string): string | undefined {
  return SPACE_PATH.exec(path)?.[1]
}

/** A path without its space prefix. An unprefixed path is returned as is. */
export function stripSpace(path: string): string {
  const m = SPACE_PATH.exec(path)
  if (!m) return path
  return m[2] || '/'
}

/**
 * A path under a space. Leaves external URLs, already-prefixed paths and API
 * paths alone, so it is safe to apply to any href the config produced.
 */
export function withSpace(path: string, space: string | undefined): string {
  if (!space || !path.startsWith('/') || path.startsWith('//')) return path
  if (spaceOf(path) || path.startsWith('/api/')) return path
  return `/s/${space}${path === '/' ? '' : path}`
}

/**
 * The localStorage key of the space this browser was last in, per project
 * base, so two projects on one origin each keep their own.
 */
export function lastSpaceStorageKey(): string {
  return `rela:last-space:${relaBase() || '/'}`
}

/**
 * The space this browser was last in, or undefined. Only a hint: the server
 * falls back to the first space the principal may enter when it is unknown
 * or not permitted.
 */
export function lastSpace(): string | undefined {
  try {
    const id = localStorage.getItem(lastSpaceStorageKey()) ?? ''
    return spaceOf(`/s/${id}`) ? id : undefined
  } catch {
    return undefined
  }
}

function rememberSpace(id: string): void {
  try {
    localStorage.setItem(lastSpaceStorageKey(), id)
  } catch {
    // A refused write (private mode, full storage) only loses the hint.
  }
}

/**
 * Where the app root opens (TKT-JEAR2UA): the home of the current space.
 *
 * `spaces` are the ones the principal may enter and `current` the space the
 * server resolved. Without spaces, and for a space without a usable home,
 * this is `/dashboard`, where the root always went before spaces.
 */
export function homeTarget(spaces: SidebarSpace[], current: string | undefined): string {
  if (!current) return '/dashboard'
  const home = spaces.find((s) => s.id === current)?.home
  // A home of "/" would land back on the root and redirect forever.
  return withSpace(home && home !== '/' ? home : '/dashboard', current)
}

/**
 * The spaces this principal may enter, and the one they are in (TKT-GNKR5H).
 *
 * Fed by the `_sidebar` response, which resolves the current space on the
 * server: an unknown or hidden space falls back to the first one the principal
 * may enter. Without `spaces:` in the config the list is empty and every path
 * stays unprefixed, exactly as before spaces existed.
 */
export const useSpaceStore = defineStore('space', () => {
  const spaces = ref<SidebarSpace[]>([])
  const current = ref<string | undefined>(undefined)
  const create = ref<SidebarCreate[]>([])
  // Whether a sidebar response has been applied. Tells "no spaces" apart
  // from "not loaded yet", so the root can redirect without a request.
  const loaded = ref(false)

  const enabled = computed(() => spaces.value.length > 0)
  const currentSpace = computed(() => spaces.value.find((s) => s.id === current.value))

  function set(data: { spaces?: SidebarSpace[]; space?: string; create?: SidebarCreate[] }) {
    spaces.value = data.spaces ?? []
    current.value = data.space || undefined
    create.value = data.create ?? []
    loaded.value = true
    if (current.value) rememberSpace(current.value)
  }

  /** The href of a path in the current space. */
  function href(path: string): string {
    return enabled.value ? withSpace(path, current.value) : path
  }

  return { spaces, current, create, loaded, enabled, currentSpace, set, href }
})
