---
id: RR-GU8EL
type: review-response
title: 'Navigation guard: next(false) + router.push(to.fullPath) breaks replace/popstate semantics'
finding: 'Plan calls next(false) then router.push(to.fullPath) on confirm. (1) If the original navigation was router.replace (used widely in this codebase via useUrlFilterSync etc.), router.push corrupts history. (2) For browser back/forward (popstate), the cursor has already moved; pushing the same URL forward inverts history. (3) skipDirtyGuard flag is a leak waiting to happen. The Vue Router 4 idiomatic pattern is to await inside the guard and return the boolean — guard functions support async. Switch to: onBeforeRouteLeave(async () => { if (!dirty.value) return true; const ok = await ask(...); if (ok) dirty.value = false; return ok }).'
severity: critical
resolution: Plan updated to use Vue Router 4 await-in-guard pattern. The guard returns the boolean directly; no next(false)+router.push, no skipDirtyGuard flag. Replace and popstate semantics are preserved by the router itself.
status: addressed
---
