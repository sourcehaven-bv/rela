---
id: RR-HTS2Q
type: review-response
title: Cmd+K idempotency when palette is already open
finding: 'Plan replaces the // TODO with paletteOpen.value = true but doesn''t specify behavior when Cmd+K is pressed while the palette is already open. Two reasonable behaviors: (1) no-op (recommended, matches ''?'' for shortcuts modal) or (2) toggle off. Lock one in with a test. Also pin behavior for Cmd+K while a ConfirmModal is open: palette opens on top (allowed; supported by Teleport + modalStack). Add tests for both cases. Implementation gives no-op for free since paletteOpen.value = true is idempotent — but the test prevents future regressions.'
severity: significant
resolution: 'Plan updated to specify no-op semantics when Cmd+K is pressed while the palette is already open (paletteOpen.value = true is idempotent — no watcher re-fire). Edge-cases section adds two tests: (1) press Cmd+K twice in sequence and assert the modal isn''t re-mounted (no transition), and (2) open ConfirmModal then press Cmd+K and assert palette opens on top with both registered in the modal stack.'
status: addressed
---
