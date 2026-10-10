import type { PileSummary } from '@/api/piles'

/*
 * Pile names by id, as last listed. Scope navigation labels the entity page
 * with the pile's name and runs outside any query, so it reads the name here.
 * Module state, because the sidebar's query is what fills it.
 */
const knownNames = new Map<string, string>()

/** The name of a pile this session has listed, if any. */
export function knownPileName(id: string): string | undefined {
  return knownNames.get(id)
}

/** Records pile names as listed. */
export function rememberPileNames(piles: readonly Pick<PileSummary, 'id' | 'name'>[]): void {
  for (const p of piles) knownNames.set(p.id, p.name)
}

/** Forgets a deleted pile. */
export function forgetPileName(id: string): void {
  knownNames.delete(id)
}

/** Test seam: reset module state between cases. */
export function resetPileNames(): void {
  knownNames.clear()
}
