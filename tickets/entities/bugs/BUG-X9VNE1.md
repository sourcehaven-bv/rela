---
id: BUG-X9VNE1
type: bug
title: API error messages discarded at 22 call sites (interceptor rejects plain objects)
description: 'The shared API client''s response interceptor (frontend/src/api/client.ts) rejects with three different shapes: a plain script_error envelope, a plain ProblemDetail object, or the raw AxiosError. 22 call sites across 12 files use `err instanceof Error ? err.message : ''<generic>''` — wrong in every branch: the plain objects fail instanceof (server detail/title discarded), and the AxiosError''s message is axios''s generic ''Request failed with status code N''. Users see ''Failed to update entity'' instead of the server''s actual validation/policy message. Four compensating parsers each re-derive shape knowledge (useAutoSave.parseError, DynamicForm duck-typing, usePageData.isCancelledFetch, KanbanView casts).'
priority: high
effort: m
why1: 'Consumers use `err instanceof Error ? err.message : ''<generic>''`, but the interceptor rejects structured API errors as plain objects (ProblemDetail / script envelope) which fail instanceof — and for raw AxiosErrors the .message is axios''s generic status-code string. The server''s detail/title never reaches the UI on any branch.'
why2: The interceptor was written to pass server payloads through unchanged for downstream branching (script_error routing) instead of normalizing to one Error type at the boundary, so the rejection shape depends on the failure class.
why3: There is no project convention for the error boundary contract — each consumer (and each of the four compensating parsers) independently guessed the shape, and the `instanceof Error` idiom looks correct in review, so it spread by copy-paste.
why4: No test asserts what a consumer receives when the API fails — unit tests mock api/* modules above the interceptor, so the boundary's rejection shape was never pinned by a contract test.
why5: Cross-cutting infrastructure (error normalization, like cache invalidation and conflict handling) had no owner or design note; pieces were added per-feature without anyone holding the boundary contract. The frontend review (2026-06-09) now records these contracts; this fix establishes the typed-error convention.
prevention: Contract tests in frontend/src/api/errors.test.ts pin the rejection shape for every failure class (the gap why4 identified). One getErrorMessage() helper replaces the instanceof-Error idiom so there is no shape knowledge left to copy-paste wrong; errors.ts documents the catch-site conventions in its header comment. The four divergent parsers are deleted, removing the drift surface.
status: done
---

Found in the 2026-06-09 frontend architecture review (finding A6). Fix:
normalize every failure path to one `ApiError extends Error` (kind:
script/http/cancelled/network, status, typed problem, validationErrors, script
envelope) thrown from the interceptor; shared `getErrorMessage(err)`; delete the
four divergent parsers; `isScriptError`/`isCancelledFetch` keep their call sites
but delegate to the typed error.
