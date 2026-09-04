---
id: TKT-5SZG2L
type: ticket
title: 'Operator-configurable world chrome: note text and where "go to" leads'
kind: enhancement
priority: medium
effort: m
status: backlog
---

Atlas worlds issue 6 (`atlas-world/docs/rela-worlds-issues.md`). After
TKT-SLFURL the web app's world chrome is small: the operator `banner:`, a note
on lists/boards of faced types, a note on a detail page showing a non-bare face
the caller may not write, and the "no face in this world" banner with its "Go to
<bare face>" button. All of the fixed text is rela-internal wording ("face",
"world", "default") and reads as jargon to an ISMS reader.

## Requirement

1. Per world, operator-declared templates replacing the fixed notes, with
placeholders (`{face}`, `{bare_face}`, `{world}`); empty means no note; rela's
text stays as the fallback.

   ```yaml
   worlds:
     actueel:
       select: [vastgesteld, concept]
       otherwise: default
       messages:
         read_only: 'Dit is de vastgestelde versie. Bewerken doe je in het concept.'
         absent:    'Dit item heeft nog geen vastgestelde versie.'
   ```

2. An operator-declared target for the absent case: another world or a
specific face, as a button or an automatic redirect (`on_absent: { redirect:
default }`).

3. Per copy definition, an optional `on_success.message` and landing
(`world:` / `face:` / `stay`), replacing the default "land on the face written"
rule where the workflow wants otherwise (a translator stays on the source).

Keep the safety argument: an operator who declares nothing gets today's
behaviour.

## Address-grammar hardening carried over from the TKT-SLFURL review

- Parse the address ONCE in `handleV1DynamicRoutes` and pass `entityRef`
(not `string`) to every entity handler, so a handler cannot forget to parse
(RR-X2GBWJ). Add a guard test over `store.GetEntity` / `reader.getEntity` call
sites and a round-trip test over every emitted address (`_self`, `_faces[].ref`,
`_attachments[].href`).
- Export of a non-bare face (`/_export` on `ID@face`) — today the uniform
not-found; the redacting `visibility.Reader.Get` reads by bare id.
- `/_documents/{name}/{id}` rejects `@` with a 400 while addresses 404
elsewhere; documents are per entity, decide whether to accept the address and
strip it or keep refusing uniformly.
