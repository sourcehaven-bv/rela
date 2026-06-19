---
id: RR-BA1YCP
type: review-response
title: AppHostView load/teardown is racy on app-switch and not cancellation-safe
finding: 'AppHostView.vue:52-67: the iframe has no :key so Vue reuses the element across app switches; the ''if (!srcdoc.value) return'' guard doesn''t distinguish old-vs-new document. If getAppHtml for a new app rejects, srcdoc='''' and the stale iframe keeps the old doc with a torn-down port (old app wedged). Also: loadApp never passes an AbortSignal to getAppHtml (apps.ts:15 accepts one) — navigating away mid-fetch resolves on a dead component and sets srcdoc. FIX: add :key=appId to the iframe (fresh element per app removes the stale-doc/torn-port race class) and pass an AbortSignal so navigation aborts the in-flight fetch.'
severity: significant
resolution: 'AppHostView.vue now: (1) bumps an appKey ref each load and binds :key=appKey on the iframe so each app gets a fresh element (no stale load on a reused element); (2) creates an AbortController per load, aborts the prior one + on unmount, passes signal to getAppHtml, and guards srcdoc/error/loading assignment on !signal.aborted so a late fetch can''t set state on a superseded/dead view. e2e still green.'
status: addressed
---
