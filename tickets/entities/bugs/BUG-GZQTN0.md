---
id: 'BUG-GZQTN0'
type: 'bug'
title: 'The e2e harness pins its schema with URLSearchParams, so every postgres e2e run connects as "+search_path"'
description: 'pgDsnForSchema in e2e/tests/fixtures.ts built the server''s RELA_DATABASE_URL with u.searchParams.set("options", `-c search_path=${schema},public`). URLSearchParams encodes a space as "+", and a URI query reads "+" as a literal plus, so the DSN went out as options=-c+search_path%3D... and every rela-server start failed with FATAL: unrecognized configuration parameter "+search_path" (SQLSTATE 42704). The whole postgres e2e job fails at server startup, not in any individual spec. This is the same defect as BUG-P1QKMB one language over: that fix corrected the Go side (ensurePoolFloor and four test helpers) and the Go suites went green, which made the remaining TypeScript instance look like an unrelated E2E failure on the pgx bump PR.'
priority: 'high'
why1: 'pgDsnForSchema set the libpq `options` parameter through URLSearchParams, which encodes the space in `-c search_path=...` as "+".'
why2: 'A URI query reads "+" as a literal plus, so libpq and pgx both parse the parameter name as "+search_path" and the server rejects the connection.'
why3: 'The fix for the identical Go defect (BUG-P1QKMB) was scoped by language rather than by defect class. Go was searched for the pattern; the e2e harness builds the same DSN in TypeScript and was not.'
why4: 'pgx before v5.11.0 decoded "+" back to a space, so this DSN connected and the e2e job passed. The TypeScript instance had no symptom to find until the pgx bump removed the compensation, and by then the Go fix had already been credited with the problem.'
why5: 'Nothing ties the two DSN builders together. They target the same server, encode the same parameter and share the same hazard, but they live in different languages and no test or lint asserts the property across both, so a fix in one cannot prompt a check of the other.'
prevention: 'The harness now pins the schema with a plain `search_path` runtime parameter instead of `options=-c search_path=...`. That value contains no space, so there is nothing for any encoder to get wrong, and it is the same form internal/jobs/pgqueue_test.go settled on for the Go side. Verified against pgx v5.11.0 directly: the old encoding parses as "-c+search_path=..." and the new one as "search_path" -> "schema,public". Generalizable lesson: when a defect is a property of a wire format rather than of a language, search for it by the format''s shape across every language in the repo, not by the language the first instance was found in.'
status: 'done'
---
