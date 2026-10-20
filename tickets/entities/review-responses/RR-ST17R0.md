---
id: RR-ST17R0
type: review-response
title: The library components reach for app-only collaborators
finding: '§2 and §4: RlChannelComposer uses `EntityPickerModal` (frontend/src/components/forms, app side), enum labels "from the schema store", and `<PendingButton>` (that component does not exist; it is `RlButton` with `pendingLabel`). RlRefChip "links to the entity", which needs the router or URL. The stories get the strict renderer "through a small story helper", which means importing app code. `scripts/check-purity.mjs` forbids every one of these (imports only from Vue and the library, no URL). A presentational component also cannot know when the request it emitted has finished. Fix: put these in the component contract. Give the composer a ref-picker slot or `pickRef: (to) => Promise<Ref|null>` prop, `enumOptions` as props, `entityHref: (ref) => string` (or emit `open-ref`), and `pending: Set<actionKey>` from the app. The stories pass a text-only renderer (escape plus `<p>`), and the real sanitizer is tested in the app.'
severity: significant
resolution: The library components take pickRef, enumOptions, entityHref, pending and the renderer as props, use RlButton with pendingLabel, and stories use a text-only renderer, so the purity check stays green (R9).
status: addressed
---
