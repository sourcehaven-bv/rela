// Module-level registry tracking which forms are currently editing which
// entities, so that incoming SSE entity:updated events don't clobber a
// user's in-progress keystrokes for any property they're editing.
//
// Multiple forms may register for the same entity (e.g. side panel + main
// page); a property is considered dirty if any registered form reports it
// dirty. Each form receives its own unregister callback for clean teardown.
//
// See TKT-18JS6 (RR-Z5PQ2).

export type DirtyCheck = (property: string) => boolean

const registry: Map<string, Set<DirtyCheck>> = new Map()

export function registerForm(entityId: string, check: DirtyCheck): () => void {
  let checks = registry.get(entityId)
  if (!checks) {
    checks = new Set()
    registry.set(entityId, checks)
  }
  checks.add(check)
  return () => {
    const set = registry.get(entityId)
    if (!set) return
    set.delete(check)
    if (set.size === 0) registry.delete(entityId)
  }
}

export function anyFormDirty(entityId: string, property: string): boolean {
  const checks = registry.get(entityId)
  if (!checks) return false
  for (const check of checks) {
    if (check(property)) return true
  }
  return false
}

// For tests only: snapshot of registry state.
export function _registrySize(): number {
  return registry.size
}

export function _registryClear(): void {
  registry.clear()
}
