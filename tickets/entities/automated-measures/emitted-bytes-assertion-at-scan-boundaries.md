---
id: emitted-bytes-assertion-at-scan-boundaries
type: automated-measure
title: 'Assert on emitted bytes, not the decoded value, wherever a scan runs between writer and parser'
kind: test
location: internal/markdown, internal/store/fsstore (frontmatter emit -> conflict scan)
status: proposed
description: >-
  The sibling measure [[yaml-roundtrip-property-test]] says to marshal,
  unmarshal and COMPARE rather than check for an absent error. BUG-TOXQAA is
  the case that passes that bar and still corrupts: a property key beginning
  with git's conflict marker round-trips through yaml.v3 perfectly, so a
  marshal-unmarshal-compare oracle is green on exactly the input that makes
  the file unreadable.

  The reason is a step the oracle cannot see. rela runs its own line-anchored
  conflict scan over the assembled file BEFORE parsing it, so the file the
  store just wrote is refused on every later read -- and, because that scan is
  also what excludes a file from the validator and the search index, the
  entity silently disappears from both.

  The measure: where a scan, filter or precondition runs between the writer
  and the parser, the test must assert on the EMITTED BYTES against that same
  predicate. Round-tripping through the serializer proves the serializer is
  self-consistent; it proves nothing about a rule applied to the bytes in
  between.
---

## Why a round-trip oracle is not enough here

`yaml.Marshal` and `yaml.Unmarshal` agree completely about
`map[string]any{"<<<<<<<": "v"}`. Both directions are correct, so
`reflect.DeepEqual(got, data)` passes. The defect is that the correct YAML
they agree on contains a line that rela itself refuses:

```text
 <<<<<<<: v
 id: T-1
```

(indented one space here for the same reason this file cannot contain the
marker at column 0.)

The oracle's blind spot is structural, not an oversight: it compares a value
to a value, and the failure happens to a file.

## Where this applies

Any place rela inspects raw bytes it produced, before or instead of parsing
them:

- `markdown.HasConflictMarkers` / `fsstore.hasLineAnchoredConflict` over
  frontmatter+body — covered as of BUG-TOXQAA
  (`TestMarshalOrdered_ConflictMarkerKeyIsNotWrittenAtColumnZero`).
- `frontmatter.Split`'s delimiter detection — a property value or key that
  emits `---` at column 0 is the same shape. Not currently covered; the
  emitter quotes such keys for unrelated reasons, which is luck rather than a
  tested property.
- Any future pre-parse gate (size caps, encoding sniffing, a signature check).

## Deliberately not proposed

Making the conflict scan fence-aware, or narrowing it to a full marker line
(`<<<<<<< <ref>`). Both make the scanner cleverer about content it deliberately
does not parse — the scan runs precisely because the file may not be
parseable. BUG-WN6D already tightened this once (substring to line-anchored)
and the line-anchored rule is correct; the fix belongs on the writer, which
knows it is emitting a key.
