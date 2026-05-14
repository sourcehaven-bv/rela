---
id: RR-OCSO
type: review-response
title: as never casts on cm.on/off hide typed CodeMirror EventMap regression
finding: 'useBacktickAutocomplete.ts lines 558-570 all five `cm.on(...)` and `cm.off(...)` calls use `as never`. The composable''s `CodeMirrorLike` interface (lines 94-106) declares `on/off` with a fully-generic `(...args: unknown[]) => void` handler. The real `@types/codemirror` declares `EditorEventMap` with precise per-event signatures (e.g. `inputRead: (instance: Editor, changeObj: EditorChange) => void`, `keydown: (instance: Editor, event: KeyboardEvent) => void` via the DOMEvent union). The composable''s `onInputRead`/`onChange`/etc are typed but the casts erase that check at the subscription site. A future CodeMirror version that adds a third argument to `inputRead` (or renames the event) would compile silently and break at runtime. Same risk for keydown''s KeyboardEvent argument shape. Fix: change EasyMdeLike to wrap the real CodeMirror.Editor type from @types/codemirror, then drop the `as never` casts — the unit-test shim can still pass a structural duck, the production type-check catches drift.'
severity: significant
resolution: CodeMirrorLike now uses typed overload signatures for on/off matching each event's actual handler shape. All five `cm.on(...)` and `cm.off(...)` call sites lost their `as never` casts. The test shim casts through `unknown` once, with a comment explaining why a single generic handler suffices for the mock.
status: addressed
---
