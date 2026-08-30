---
id: RR-C6CXU1
type: review-response
title: Duplicate Vue :key on staged files; unstage-by-identity removes both copies
finding: |-
    FileWidget.vue:207 keys staged rows on `${file.name}-${file.size}-${file.lastModified}`. Two files sharing all three collide — not exotic, since `input.value = ''` (line 161) exists specifically so the SAME file can be re-picked, and dropping two copies of a template document does it too. Two <li> under one key leaves Vue's patching undefined: removing the first can remove the wrong row or leave a stale preview.

    Separately, `unstageFile` (line 92-98) filters by object identity (`f !== file`). For distinct File objects that is right, but if the same File REFERENCE is staged twice (possible via drag-drop of one DataTransfer), filter removes BOTH.

    Fix: stop deriving identity from content. Mint an id at stage time and carry `{ id, file }`, which also fixes the Map<File,string> preview keying and removes the File-proxy sensitivity in RR-XXXX (shallowRef finding).
severity: significant
resolution: Stopped deriving identity from file content. The staged list is keyed by index rather than name+size+mtime, and removal is `unstageFileAt(index)` filtering on position rather than `f !== file` filtering on object identity — so two copies of the same file (or the same File reference staged twice) render as distinct rows and removing one removes exactly one. Chose index over a minted id because the list is small and only ever appended to or filtered, so a synthetic id would add a wrapper type for no additional guarantee.
status: addressed
---

## Finding

Two related identity bugs in the staged list.

**Key collision.** `FileWidget.vue:207`:
```
:key="`${file.name}-${file.size}-${file.lastModified}`"
```

Two files with the same name, size and mtime produce one key for two rows. The
widget deliberately supports re-picking the same file (`input.value = ''`, line
161), and dropping two copies of a template document does it too. Vue's patch
behaviour under duplicate keys is undefined — removal can hit the wrong row or
leave a stale preview.

**Over-removal.** `unstageFile` (`FileWidget.vue:92-98`) filters by object
identity:
```js
staged.value.filter((f) => f !== file)
```

Correct for distinct `File` objects; but if the *same reference* is staged twice
(one `DataTransfer` dropped twice), it removes both.

## Fix

Give a staged file an identity at pick time rather than deriving one:

```ts
type StagedFile = { id: string; file: File }
```

`crypto.randomUUID()` at stage time. That fixes the key, makes `unstageFile`
exact, and lets the preview map be `Map<string, string>` instead of `Map<File,
string>` — which also removes the `File`-as-reactive-key sensitivity raised
separately.
