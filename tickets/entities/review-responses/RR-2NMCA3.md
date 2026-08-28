---
id: RR-2NMCA3
type: review-response
title: DashboardView's store-load gate reads as load-bearing but App.vue is the real gate
finding: |-
    The `if (!schemaStore.loaded) await schemaStore.load()` gate added to DashboardView's onMounted (per RR-TIO1XP) is effectively dead code in production: App.vue renders <RouterView/> only in the v-else branch after loading===false and error===null, which implies loaded===true. DashboardView cannot normally mount before the store has loaded.

    The test covering it constructs a state App.vue structurally prevents. It is not vacuous (removing the gate fails it), but it pins a scenario production cannot reach. RR-TIO1XP's empty-state-flash concern was real in the abstract; App.vue had already solved it.

    Risk: it reads as the thing keeping the flash away, so someone refactoring the boot sequence would trust it and be wrong about why.
severity: minor
resolution: 'Kept the gate as defence-in-depth (it genuinely matters when the component is mounted directly, as its unit tests do, and if the boot sequence ever changes), but rewrote the comment to say plainly what it is: ''Belt-and-braces, NOT the thing preventing the empty-state flash: App.vue renders <RouterView/> only after the store has loaded...''. The next person reading it now learns where the real gate lives instead of inheriting a false belief.'
status: addressed
---
