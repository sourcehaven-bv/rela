---
id: RR-C1MI2N
type: review-response
title: 'Deep ref proxies File under happy-dom: tests exercise proxies, production exercises raw Files'
finding: |-
    `stagedFiles` is `ref<Record<string, File[]>>` — a DEEP ref. Vue skips proxying by `toRawType`; a spec File reports 'File' and is left alone. But vitest runs happy-dom, whose File reports '[object Object]', so Vue proxies it.

    Verified empirically in this repo (throwaway test): under happy-dom `Object.prototype.toString.call(file)` === '[object Object]', `isProxy(deep.value.p[0])` === true, and `deep.value.p[0] === f` === FALSE. Identity is not preserved. With shallowRef both are correct (not proxied, identity preserved).

    So the unit tests exercise File PROXIES while production exercises raw Files, silently changing the behaviour of three things this feature depends on: `unstageFile`'s `f !== file` identity filter, `stagedPreviews` (a Map keyed BY File), and the brand checks in URL.createObjectURL / FormData.append.

    It happens to work in both today — luck, not design. Nothing needs deep reactivity on a File: `updateStagedFiles` already replaces the whole object on every change, so shallowRef is sufficient AND makes test and production agree. Same for `stagedPreviews` (FileWidget.vue:48), whose Map never needs reactive tracking — the watch(staged, ...) drives revocation.
severity: significant
resolution: 'stagedFiles is now shallowRef, and stagedPreviews likewise. Verified the underlying claim independently with a throwaway test: under happy-dom a File reports ''[object Object]'', a deep ref proxies it, and identity is NOT preserved (deep.value.p[0] === f is false); shallowRef preserves both. With the change, test and production now agree on File identity, so the index-based removal in RR-C6CXU1 and the Map keying are exercising the shipped semantics rather than a proxy artefact.'
status: addressed
---

## Finding

The unit tests cannot observe production's actual object semantics.

`stagedFiles` is a **deep** `ref`. Vue decides whether to proxy via
`toRawType(target)`: a spec `File` reports `"File"` → `INVALID` → untouched. But
`vitest.config.ts` uses `environment: 'happy-dom'`, whose `File` reports
`[object Object]` → proxied.

## Verified in this repo

A throwaway test (`src/proxycheck.test.ts`, since removed):

```
tag:               '[object Object]'
proxied:           true
identityPreserved: false      ← deep.value.p[0] === f is FALSE
```

With `shallowRef`: not proxied, identity preserved.

## Why it matters

Three things this feature relies on behave differently on a proxy than on a raw
`File`:

- `unstageFile`'s `f !== file` identity filter (`FileWidget.vue:92-98`)
- `stagedPreviews`, a `Map<File, string>` whose **keys** are proxies in test and
raw objects in production (`FileWidget.vue:48`)
- `URL.createObjectURL(file)` and `FormData.append('file', file)`, which brand-check

It works in both today, but by luck. The tests are structurally unable to catch
a regression in exactly the area the identity bugs live (see RR-C6CXU1).

## Fix

Nothing here needs deep reactivity over a `File`. `updateStagedFiles` already
replaces the whole object on every change, so `shallowRef` is sufficient and
makes test and production agree:

```ts
const stagedFiles = shallowRef<Record<string, File[]>>({})
```

Same for `stagedPreviews` — the `watch(staged, …)` drives revocation, not Map
reactivity.
