/**
 * Ranking for the `@` completion menu's results.
 *
 * Its own module, and generic over anything carrying an `id`, because the
 * sandboxed app editor uses the same ranking from a plain IIFE bundle. Reaching
 * into `useMentionMenu` for it would pull Vue and the axios API layer into that
 * bundle, neither of which it can use.
 */

/**
 * Ranks exact and prefix ID matches ahead of loose title matches.
 *
 * The search backend scores by title-token relevance, so typing an ID prefix
 * buries the entity the user is plainly aiming at. Ranking is stable within a
 * tier, so the backend's relevance order survives as the tie-break.
 */
export function rankByIdMatch<T extends { id?: string }>(items: T[], query: string): T[] {
  if (!query) return items
  const q = query.toUpperCase()
  const tier = (e: T): number => {
    const id = (e.id ?? '').toUpperCase()
    if (id === q) return 0
    if (id.startsWith(q)) return 1
    if (id.includes(q)) return 2
    return 3
  }
  return items
    .map((e, i) => ({ e, i, t: tier(e) }))
    .sort((a, b) => a.t - b.t || a.i - b.i)
    .map(({ e }) => e)
}
