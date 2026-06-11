---
id: BUG-DTE2FF
type: bug
title: DynamicForm dirty-registry cleanup never runs (onBeforeUnmount after await)
description: 'Every edit-form mount leaked its DirtyCheck closure into dirtyFormRegistry: the onBeforeUnmount(unregister) call sat inside async onMounted after several awaits, where Vue has no active component instance, so the hook was silently dropped. A form unmounted while a property was dirty would permanently report that property dirty for its entity, suppressing the SSE merge path the registry exists to control (TKT-18JS6). Fixed in PR #946 by storing the unregister fn at setup scope and calling it from the existing top-level onBeforeUnmount. The PR bundles two further quick fixes from the frontend review: HelpModal now sanitizes /api/help HTML with DOMPurify like every other v-html sink, and Kanban swimlane cards gate draggable on canUpdate() like the simple board (failed drops now toast the server message).'
priority: medium
effort: s
why1: onBeforeUnmount(unregister) was invoked inside async onMounted after awaits; Vue can only associate lifecycle hooks with an instance during synchronous execution, so the hook was silently dropped (dev-only console warning).
why2: The registration depends on values produced by the awaited loads (entityId, formConfig, the autosave instance), so it was written inline at the point those values exist, without realizing hook registration itself must stay synchronous with setup.
why3: No test exercised a mount/unmount cycle of an edit form against the registry, and vue/no-lifecycle-after-await only flags awaits at setup() top level, not inside an onMounted callback — so neither CI nor lint caught it.
why4: The registry's consumer side (anyFormDirty in the SSE merge path) was never wired up, so the leak had no observable symptom that would have surfaced it in use.
why5: Cross-cutting mechanisms landed half-built (registry without consumer, ETag plumbing without population) fail invisibly; landing mechanism and consumer together — or tracking the missing half as an explicit ticket — removes the class.
prevention: Regression test in dirtyFormRegistry.test.ts replicates the async-onMounted registration pattern and asserts the registry empties on unmount. Comment at the registration site documents why cleanup must route through the top-level onBeforeUnmount. The unwired consumer side (SSE merge / anyFormDirty) is recorded in the frontend review as an explicit follow-up so the half-built mechanism is tracked rather than latent.
status: done
---

Found during the 2026-06-09 frontend architecture review. Fixed in PR #946
together with two sibling review findings (HelpModal v-html sanitization, Kanban
swimlane affordance gate).
