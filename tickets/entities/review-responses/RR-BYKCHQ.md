---
id: RR-BYKCHQ
type: review-response
title: Faced type with no bare_face is unpinned; a future first-face fallback would render a wrong legal status
finding: 'loader.go:709-711 permits `faces:` with no `bare_face:` (docs call it ''legal but rarely intended''). For such a type on a bare-addressed row, onScreenFace resolves to '''' via `?? ''''` and noticeNote correctly returns ''''. Nothing pins this. The failure mode if it regressed is severe: if someone replaced the fallback with a first-declared-face scan (the exact thing DeclaredFace at copies.go:369-371 warns was removed because ''the answer depended on map order''), an arbitrary face''s notice would render on a row in no face — a wrong statement about a document''s legal status, the precise risk class this ticket exists to address. EntityDetail.world.test.ts:1201 already covers a faced type with no bare_face, so the fixture pattern exists.'
severity: minor
resolution: 'Added ''says nothing on a bare row of a faced type that names no bare_face'', seeding a type with faces but no bare_face and a notice on each face, asserting no banner. Carries a positive control per the file''s convention: the same fixture WITH a bare_face does render the notice, so the absence is about the missing bare_face rather than an inert fixture. Mutation-verified against the specific regression named: replacing the fallback with a first-declared-face scan (`|| Object.keys(faces)[0]`) fails this test and only this test.'
status: addressed
---
