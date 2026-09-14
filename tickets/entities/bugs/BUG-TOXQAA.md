---
id: BUG-TOXQAA
type: bug
title: A conflict-marker property name makes fsstore write a file it can never read back
description: 'A property NAME beginning with git''s opening conflict marker ("<<<<<<<") is written by fsstore as a plain YAML mapping key. Keys are emitted at column 0, so the file fsstore has just written scans as an unresolved merge and every later read refuses it with ErrConflictedFile -- the entity is created successfully and is then unreadable, silently absent from the validator and the search index. memstore and pgstore round-trip the same entity fine, so the backends disagree about what an entity IS. A second, unrelated defect found in the same sweep: metamodel.ValidateIDPrefix accepts an id_prefix leading with "_" or "-", which mints IDs ("_-276F18") that entity.ValidateID then refuses. Both found by the weekly fuzz sweep (issue #993), targets FuzzCloneNestedValues and FuzzGenerateShortID.'
priority: medium
why1: fsstore's markdown emitter wrote every frontmatter key as a plain YAML scalar. A mapping key is emitted at column 0, so a key beginning with "<<<<<<<" produced a line indistinguishable from git's opening conflict marker, and the file-level conflict scan that runs before parsing refused the file on every subsequent read.
why2: The round-trip guards added for BUG-B1RA3J (KeyNode, ValueToNode) were scoped to what yaml.v3 itself cannot read back. This key round-trips through YAML perfectly; the breakage is one level up, at rela's own file-level conflict scan, so the existing key guard had no reason to consider it.
why3: The key guard's round-trip test asserted marshal-unmarshal equality against yaml.v3 only. That oracle cannot see a defect introduced by a scan rela performs on the assembled file, so the test passed on exactly the input that corrupts.
why4: Two layers each hold a rule about column 0 -- the YAML emitter decides where a key lands, the conflict scanner decides what column 0 means -- and neither owned the interaction. The emitter had no reason to know a marker at column 0 is special to rela, and the scanner had no reason to know rela itself generates that line.
why5: 'Serialization correctness was defined as "yaml.v3 reads back what it wrote" rather than "the store reads back what the caller wrote". The narrower oracle is the systemic cause: it is blind to any transformation between the emitter and the parser, and rela has such a step. The fix moves the assertion to the emitted BYTES and to a cross-backend conformance case, so a store that cannot read back its own write fails a test instead of losing an entity.'
prevention: The markdown regression test asserts on the emitted bytes (HasConflictMarkers over the frontmatter), not on the decoded map, so any future emitter change that lands a marker at column 0 fails. A storetest conformance case pins that every backend round-trips such a property name, so a new store inherits the requirement. Both FormatDocumentOrdered fallbacks now route through marshalOrdered, closing the empty-keyOrder path that bypassed the key guards entirely.
status: done
---

## Description

Two defects from the weekly fuzz sweep
([#993](https://github.com/sourcehaven-bv/rela/issues/993)). Both are
write-then-cannot-read.

### 1. Conflict-marker property name (fsstore)

`FuzzCloneNestedValues` with property name `"<<<<<<<"`:

```text
go test fuzz v1
string("<<<<<<<")
int(397)
```

fsstore serializes the frontmatter with `yaml.Marshal`, which puts a mapping key
at column 0:

(indented one space, because rela's own conflict scan is line-anchored and
would otherwise refuse THIS file — the defect demonstrating itself):

```yaml
 ---
 <<<<<<<:
     - a
 id: T-1
 ---
```

`parseDocument` runs `hasLineAnchoredConflict` over the raw file before parsing,
so the file fsstore has just written is refused on every read:

```text
file has unresolved git conflicts
```

The entity is created successfully and is then unreadable. Because the conflict
scan is also what excludes a file from the validator and the search index, the
entity silently disappears from both.

memstore accepts the same entity and round-trips it fine. So the backends
disagree about what an entity is — the same divergence class as BUG-X7ICNM.

Scope is narrow and worth stating: only a key **beginning** with the marker
breaks. `"x<<<<<<<"` is fine (the emitter puts it at a non-zero column anyway),
`"======="` and `">>>>>>>"` are fine (the scanner only looks for the opening
marker, per BUG-WN6D's line-anchoring), and **values** are fine at any nesting —
yaml.v3 always writes a value after `"key: "` or indented under a block scalar,
never at column 0.

### 2. Leading `_` or `-` in an `id_prefix` (metamodel)

`FuzzGenerateShortID` with prefix `"_"`:

```text
go test fuzz v1
string("_")
string("")
int(10000)
string("0")
```

`metamodel.ValidateIDPrefix` was hardened for `"--"` in BUG-RHFHTH but never for
the first character. It accepts `"_"`, so `GenerateShortID` mints `_-276F18`,
which `entity.ValidateID` then refuses:

```text
entity ID must start with a letter or digit: _-276F18
```

`GenerateShortID`'s godoc already states its precondition is a load-validated
prefix. The prefix validator simply did not enforce the whole of `ValidateID`'s
grammar.

## Fix direction

**Quote, don't reject** — the same call BUG-B1RA3J made, and for the same reason
recorded there: the value is representable, so a serialization limit must not
become a data-validity rule. Refusing a conflict-marker property name in
`storeutil.ValidateProperties` would make pgstore and memstore reject an entity
they store correctly today, to work around a limitation neither has.

The fix belongs in `markdown.KeyNode`, beside the guards already there, and
applies at all four call sites. Double-quoting moves the marker off column 0:

```yaml
"<<<<<<< HEAD": v
```

For the prefix, a load-time gate matching the `"--"` rule beside it: the failure
names the offending `schema.yaml` line rather than surfacing later as a rejected
write. `"a_b-"` stays valid — an underscore is legal in an ID, just not first.
No shipped schema uses an affected prefix.

## Severity

Low likelihood, bad failure mode. A property named `<<<<<<<` is not something a
user types; the realistic routes are an importer mapping a column header, a Lua
automation building a key from data, or a sync peer. But the outcome is an
entity that reports a successful write and then does not exist as far as reads,
validation and search are concerned — with no error anywhere pointing at the
cause.

The prefix defect is lower still (an operator would notice at the first create),
and is fixed here because it is a one-line gate in the same sweep.

## Verification

Assert a round trip, and assert it at the level where the defect lives. The
markdown test checks the emitted **bytes** against `HasConflictMarkers` rather
than the decoded map, because the key round-trips through YAML perfectly — a
map-equality oracle passes on the corrupting input. A `storetest` conformance
case pins that all backends agree. Both were confirmed to fail without the fix.
