---
id: RR-VGS4J
type: review-response
title: Modal lifecycle pattern duplicated across ConfirmModal and CommandPaletteModal — extract
finding: 'ConfirmModal and CommandPaletteModal now share Teleport, useModalStack(computed(() => props.open)), previouslyFocused capture/restore via watcher, Escape with stopPropagation, overlay click target===currentTarget guard, random-suffix ID for ARIA labelling. Five duplicated patterns. Fix: extract useModalLifecycle({ open, onClose, focusOnOpen }) when a third modal lands. Defer; current code is acceptable, but track as follow-up.'
severity: minor
reason: Not blocking. Two duplicates is acceptable; extract on the third modal. Tracked as architectural follow-up; revisit when a third modal type lands. The current code is small enough that DRY-prematurely would obscure intent.
status: deferred
---
