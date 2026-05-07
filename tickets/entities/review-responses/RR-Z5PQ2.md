---
id: RR-Z5PQ2
type: review-response
title: 'Module-level dirty registry: HMR concerns and Map<entityId, single-callback> pattern breaks with two form instances'
finding: |-
    Plan: 'tiny module-level registry dirtyFormRegistry: Map<entityId, ...>'. Two issues:

    (1) HMR: Vite HMR can re-execute module code without remounting components, leaving stale registrations. (Vue 3 lifecycle hooks fire correctly under HMR, but registry state needs unit-test coverage of repeated mount/unmount cycles.)

    (2) Two DynamicForm instances on the same entityId (side panel + main page, or modal-edit) — single-key Map silently overwrites. First form's dirty fields no longer protected; on unmount the second form clears the entry, leaving the first half-state.

    Fix: Map<entityId, Set<{isDirty: (prop) => boolean}>>. SSE consumer skips refresh iff *any* registered form reports the property dirty. On unmount remove only this form's callback. Test: mount/unmount 5× and assert empty registry.
severity: minor
resolution: 'Registry shape is Map<entityId, Set<DirtyCheck>>. registerForm returns its own unregister callback so multiple forms on the same entityId coexist; SSE refresh skips a property iff *any* registered callback reports it dirty. AC #13 mounts two forms on the same entity to verify. HMR coverage via mount/unmount cycle test.'
status: addressed
---
