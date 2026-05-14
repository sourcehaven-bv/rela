---
id: RR-SKX3
type: review-response
title: 'Focus restoration: picker closes but editor cursor focus path is implicit'
finding: 'Plan says ''editor.codemirror.focus() then close the modal.'' CommandPaletteModal restores focus to `previouslyFocused` (the element that had focus before opening), which may NOT be the CodeMirror textarea — it could be the toolbar button that opened the picker. If we rely on CommandPaletteModal''s existing restore-focus path, the user lands on the toolbar button, not the editor. Mitigation: call `editor.codemirror.focus()` explicitly AFTER the modal emits `close`, in the MarkdownEditor parent — overriding the default restore. The plan describes this but the ordering matters: the picker''s `previouslyFocused.focus()` runs in the `open` watcher cleanup; parent''s focus call must be in `nextTick` after the modal close to win the race.'
severity: minor
resolution: 'Plan §Approach §2: parent calls editor.codemirror.focus() in nextTick after the modal closes, AFTER the modal''s previouslyFocused restore runs. AC 7 covers both select-and-close and close-without-select paths.'
status: addressed
---
