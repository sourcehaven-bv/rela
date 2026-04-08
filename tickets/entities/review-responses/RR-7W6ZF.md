---
id: RR-7W6ZF
type: review-response
title: Backspace on EntityDetail during loading opens empty modal and hijacks browser back-nav
finding: 'EntityDetail.vue handleKeydown fires Delete/Backspace without guarding on entity.value being loaded. During initial load (entity still null), the user presses Backspace expecting browser back-nav and instead gets an empty delete modal (or crash depending on template resilience). Fix: guard on entity.value being non-null before accepting the key.'
severity: significant
resolution: Added `&& entity.value` guard to the Delete/Backspace branch in EntityDetail.handleKeydown. During initial load (entity still null) the key now falls through — Backspace goes to the browser as back-nav, Delete is a no-op. Once entity loads, the shortcut becomes active.
status: addressed
---
