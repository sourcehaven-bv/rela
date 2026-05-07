---
id: RR-VKS1F
type: review-response
title: Server-side automations fire ~10× more often under auto-save; need no-op suppression and audit
finding: |-
    Per workspace.go:944-957, every successful UpdateEntity runs s.automation.Process(...). Auto-save fires PATCHes every ~800ms of typing — automations run dozens of times per session. CLAUDE.md create_entity automations have if_exists: skip but set/create_relation actions don't. Repeated status toggles fire repeated automations.

    Mitigations: (1) audit existing project automations for non-idempotent behavior; (2) add a no-op suppression in useAutoSave — if the new value equals the last-seen server value, skip the PATCH entirely.
severity: significant
resolution: 'No-op suppression in useAutoSave at debounce-fire time: if new value === lastSeenServer[prop], skip the PATCH. Bounds automation re-runs. Plan also adds an idempotence audit step against current metamodel automations (prototypes/data-entry/*/metamodel.yaml). AC #11 covers.'
status: addressed
---
