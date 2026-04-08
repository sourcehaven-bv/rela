---
id: RR-RLYNL
type: review-response
title: .modal-overlay class collision silently disables shortcuts when CommandModal is open
finding: 'Both EntityDetail.vue (line 39) and useListKeyboard.ts (line 52) guard on document.querySelector(''.modal-overlay, .shortcuts-overlay'') to decide whether to fire keyboard shortcuts. But .modal-overlay is used by CommandModal, LinkExistingModal, and InlineCreateModal in addition to ConfirmModal. Concrete breakage: when a command is running in EntityDetail (CommandModal mounted with .modal-overlay), pressing Delete/Backspace/E does nothing — the guard swallows the key even though CommandModal doesn''t own those keys. Similarly, adding any list-level .modal-overlay modal in the future will silently disable all list shortcuts (j/k/Enter/o/e/n/h/l).'
severity: critical
resolution: Introduced frontend/src/composables/modalStack.ts — an explicit Set<symbol> registry with register/unregister/isAnyModalOpen API, plus a useModalStack(openRef) convenience composable that wires registration to a reactive open ref and handles onBeforeUnmount cleanup. Wired into ConfirmModal, CommandModal, InlineCreateModal, and LinkExistingModal. EntityDetail.handleKeydown and useListKeyboard.handleKeydown now call isAnyModalOpen() instead of querying document.querySelector('.modal-overlay'). The shortcuts-overlay class query remains for KeyboardShortcutsModal which does not register with the stack yet (documented as a follow-up). Added 11 unit tests in modalStack.test.ts covering manual register/unregister, useModalStack watcher behavior, multi-modal tracking, and cleanup-on-unmount.
status: addressed
---
