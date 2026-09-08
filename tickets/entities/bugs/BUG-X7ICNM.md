---
id: BUG-X7ICNM
type: bug
title: pgstore silently substitutes U+FFFD for invalid UTF-8 where fsstore refuses
description: 'For a property value containing invalid UTF-8, fsstore correctly refuses (yaml: cannot marshal invalid UTF-8 data as !!str) while pgstore silently substitutes U+FFFD and reports success -- the write succeeds and the caller reads back a value they never wrote. The mirror image of BUG-B1RA3J: there the value was legitimate and fsstore was too strict; here it is unrepresentable and pgstore is too lax, so the fix direction reverses. Found by re-fuzzing after the BUG-B1RA3J fix.'
priority: medium
why1: pgstore and sqlitestore serialize properties with encoding/json and json.Marshal replaces each invalid UTF-8 byte with U+FFFD and returns no error so the write succeeded with a value the caller never wrote.
why2: 'No shared rule said what property TEXT a store must refuse. ID grammar had a shared oracle (storeutil.ValidateID) but text values had none so each backend inherited whatever its serializer happened to do: YAML refuses and JSON substitutes and memory keeps the bytes.'
why3: The store fuzz target asserted only that writes did not error. Silent substitution produces no error so the target could reach the corrupt case and still report a pass; BUG-B1RA3J had already shown the same blind spot for YAML block scalars.
why4: Conformance was framed as "the backends behave the same on valid input" and the definition of valid input was left to each serializer instead of being owned by the store contract.
why5: 'Validity of stored text was owned by nobody: each serializer decided for itself and no conformance test compared what came back to what went in. The systemic fix is a store-owned definition of valid property text plus a round-trip oracle every backend runs so the next divergence fails a test instead of corrupting a row.'
prevention: One text rule (storeutil.ValidateProperties) called from every backend write path and used as the directional oracle in the fuzz target which now round-trips and COMPARES every accepted value. Any future backend inherits both through storetest.
status: done
---

## Description

For a property value containing invalid UTF-8, the two backends disagree — and
pgstore is the unsafe one.

```go
// fsstore (YAML)
yaml.Marshal("\n\xc80")
// error: yaml: cannot marshal invalid UTF-8 data as !!str   <- refuses

// pgstore (JSON)
json.Marshal(map[string]any{"p": "\n\xc80"})
// err=<nil>, out={"p":"\n�0"}                          <- SILENTLY corrupts
```

`encoding/json` replaces the invalid byte with U+FFFD (the replacement
character) and reports no error. The write succeeds, and the value the caller
gets back is not the value they wrote.

Found while fixing BUG-B1RA3J: 90s of `go test -fuzz` on
`FuzzPropertyValuesTypeZoo` after that fix surfaced this as the next failure.
Crashing input NOT committed as a corpus file — on its own it is a
deliberately-failing seed, so adding it to `testdata/` would redden the suite
before the fix exists. Recorded here instead, so the reproducer travels with the
bug:

```text
go test fuzz v1
string("0")
int(178)
string("\n\xc80")
```

The third value is the payload: `\xc8` starts a 2-byte UTF-8 sequence and `0` is
not a valid continuation byte, so the string is not valid UTF-8. Write it to
`testdata/fuzz/FuzzPropertyValuesTypeZoo/` when fixing, and it becomes the
regression seed.

## Why this is the opposite of BUG-B1RA3J

Worth stating, because the two look similar and the right answers differ.

In BUG-B1RA3J the value was legitimate, fsstore could not serialize it, and
pgstore could — so the fix was to make fsstore store it, and REJECTING would
have made a storage limitation into a data-validity rule.

Here the value is NOT legitimate: invalid UTF-8 cannot be represented in YAML at
all, and arguably should not be in a text property. **fsstore's refusal is
correct.** The defect is that pgstore accepts it and mangles it instead of
failing, which is the silent-data-loss direction the project avoids everywhere
else.

So the fix direction is the reverse: bring pgstore up to fsstore's strictness,
not the other way round.

## Scope

IN: reject invalid UTF-8 in property values at a point BOTH backends share, so
the two agree and the caller gets an error rather than a silently-substituted
value.

The shared point matters. Fixing it inside pgstore alone would leave two
independent implementations of the same rule, which is how BUG-B1RA3J's
`valueToNode` duplication happened.

OUT: making fsstore accept it. Invalid UTF-8 in a YAML text scalar is not
representable, and pretending otherwise means base64 or an escape scheme for
data nobody intends to store.

## Severity

Low in practice — invalid UTF-8 rarely reaches a property value through the UI,
which posts JSON over HTTP. More plausible through import, a Lua automation
building a string from bytes, or a sync peer.

But the failure mode is the bad one: a silent, irreversible substitution the
caller is never told about. That is worth more than its frequency suggests.

## Behaviour change

NUL (U+0000) joined the rule. It is not invalid UTF-8, but Postgres cannot hold
it in a text or jsonb column, so pgstore alone refused it — loudly — while
fsstore, sqlitestore and memstore stored it. Rather than a fuzz target that
skips NUL, the shared gate now refuses it everywhere.

That is a behaviour change for existing data on the three lax backends: a row
that already holds a NUL in a property will fail on its next UPDATE, with
`store: property "…": contains NUL`, even if the edit touched a different
property. The error names the property. Release notes should say so, because it
surfaces far from where the byte came in (`rela update -P` passes shell strings
through untouched).

## Verification

The parked seed is the regression test, once the fix lands. As with BUG-B1RA3J,
assert a ROUND TRIP rather than "no error on write" — and here also assert the
two backends AGREE, since divergence is the actual defect.
