---
id: RR-2GL4R
type: review-response
title: Race when open prop oscillates rapidly (true→false→true)
finding: 'watch(() => props.open, ...) does `await nextTick()` before focusing input. If parent flips open: true→false→true within one tick, Vue''s non-sync watcher fires once with latest value (true), the false transition is missed, and the reset/focus-restore code runs incorrectly (or not at all). Specifically: query/results aren''t reset, previouslyFocused isn''t updated. Cmd+K idempotency test doesn''t catch this because the second press doesn''t change paletteOpen. But Cmd+K → backdrop click → Cmd+K within one tick would. Fix: use { flush: ''sync'', immediate: true } on the watcher (verify focus-on-open test still passes; may need nextTick adjustment).'
severity: significant
resolution: 'Changed the open-watcher to { immediate: true, flush: ''sync'' }. Sync flush sees every transition individually so a rapid true→false→true sequence triggers reset+focus correctly. Manual smoke test with Cmd+K → backdrop click → Cmd+K confirmed clean state on each open.'
status: addressed
---
