import { relaBase } from '@/api/base'

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

let linkSpace: () => string | undefined = () => undefined

/**
 * Tells spaceHref where to find the current space. The router sets it at
 * start-up from the space store; utils cannot import the store itself without
 * a dependency cycle.
 */
export function setLinkSpace(space: () => string | undefined): void {
  linkSpace = space
}

/**
 * The address of an app path in the current space, under the project base:
 * for an href built outside the router, such as in rendered markdown.
 */
export function spaceHref(path: string): string {
  return relaBase() + withSpace(path, linkSpace()).slice(1)
}
