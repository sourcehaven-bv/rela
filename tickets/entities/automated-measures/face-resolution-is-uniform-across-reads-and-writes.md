---
id: face-resolution-is-uniform-across-reads-and-writes
type: automated-measure
title: Face resolution is the same three inputs for reads and writes
description: Pins that a write resolves and authorizes the SAME face value it writes, that ACL is applied to the resolved face (a denied face is refused, not redirected), that a faced type requires an explicit face, and that an unrecognised face key in a create body is rejected rather than dropped.
kind: test
location: internal/entitymanager/facedwrite_test.go, internal/dataentry/selfhref_test.go, internal/store/storetest/states.go
status: active
---

## What this catches

A write that resolves its coordinate by a different rule than a read. The defect
class is not "create ignores `@face`" specifically, it is that face resolution
was implemented once per operation instead of once, so the paths could disagree
without any test noticing.

## What the tests must pin

**The authorized face and the written face are one value.** `CreateEntity`
authorizes `opts.Face` and passes that same `opts.Face` into `createCoreOpts`,
so the two cannot be populated from different places. This is the invariant
that actually failed; asserting it directly is what a per-site "does this
literal set a face?" check could not do.

**A write names its face explicitly.** `?world=` is deliberately NOT a write
input (`world.go:493` refuses it): a world can answer with a FALLBACK face, so
a write riding that indirection would save the wrong state's content. On a
faced type an explicit face is REQUIRED (`ErrFaceRequired`); on a faceless type
a face is refused (`ErrFaceNotDeclared`). Reads keep the full three-input
chain, which is the asymmetry the tests must hold in place rather than
"correct".

**ACL applies to the resolved face, not the written one.** The failure mode in
BUG-HC6I2T was a redirect: the principal was refused nothing, the write simply
landed somewhere they did happen to hold. A test must assert that creating into
a face the principal cannot write is REFUSED, and assert which rule refused it.
`acl.ReadOnlyACL` cannot distinguish that from a blanket deny, so the create
tests need a scoped double in the manner of `createOnlyACL`.

**An unknown `face` key is rejected.** `handleV1CreateEntity` decodes into an
anonymous struct and `encoding/json` drops unknown keys, so the test must assert
a 4xx with a named code, not merely that the entity did not land on the face.

**Every write path, not just create.** The same authorize-here/read-there shape
was found in seven further places. `TestUpdate_ReadsThePreImageAtTheAuthorizedFace`
and `TestApply_ProbesTheFaceTheBodyNames` cover update and apply; delete and
rename go through `anyFaceOf`. Rename was the sharpest: the missed read took
the not-found branch, which skips ACL by design, so a faced rename succeeded
under a deny-all policy.

**The zero coordinate is no longer privileged.** With `bare_face` removed there
is no configuration in which a named face answers to the unsuffixed id, so the
old "put the published face at the bare row" fixture no longer expresses
anything. What replaces it is `storetest`'s
`NamedFaceNeedsNoZeroCoordinateRow`, asserting across all four backends that a
named face requires no anchor row at all.

## Why a measure rather than just tests

The asks in BUG-HC6I2T are separately satisfiable, and the create path can be
made to honour an explicit `@face` while other paths still fall through to the
zero row — which is precisely what happened in seven other places. Pinning the
INVARIANT (authorized face == written face, on every write path), rather than
each input at each site, is what makes a partial fix fail.
