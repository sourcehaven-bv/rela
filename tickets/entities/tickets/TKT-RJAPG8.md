---
id: TKT-RJAPG8
type: ticket
title: Parse the entity address once in the dispatcher; face export; /_documents address shape
kind: refactor
priority: low
effort: m
status: backlog
---

Hardening carried over from the TKT-SLFURL review (RR-X2GBWJ, RR-X9P7MW).

- Parse the address ONCE in `handleV1DynamicRoutes` and pass `entityRef`
(not `string`) to every entity handler, so a handler cannot forget to parse it.
On fsstore a raw `ID@face` string happens to hit the right index key while on
pgstore it 404s, which masks the omission locally.
- Add a guard test over `store.GetEntity` / `reader.getEntity` call sites in
`internal/dataentry` (allowlist style, like `TestNavFilterStaysPresentational`)
and a round-trip test over every emitted address (`_self`, `_faces[].ref`,
`_attachments[].href`).
- Export of a non-bare face (`/_export` on `ID@face`): today the uniform
not-found, because `visibility.Reader.Get` reads by bare id.
- `/_documents/{name}/{id}` rejects `@` with a 400 while addresses 404
elsewhere; documents are per entity, so either accept and strip the face or
refuse uniformly.
