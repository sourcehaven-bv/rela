---
id: RR-N3OU
type: review-response
title: API key leak surface broader than the one planned test covers
finding: 'Plan has one test that an HTTP failure error string does not contain the API key. Other leak sites unaddressed: (1) httputil.DumpRequest used for diagnostics will dump Authorization header; (2) panics with the request object on the stack will print headers in a recover''d %+v log; (3) developers will copy real .rela/ai.yaml into test fixtures; (4) error wrapping that includes a URL with embedded user:key@host. Fix: (a) introduce a redactKey helper used at every error construction and log site; (b) add a table-driven test that uses a poisoned API key string and asserts it appears in NO error returned from any code path across all error scenarios; (c) add a CI check or precommit hook scanning for real API key prefixes (sk-, etc.) in committed files; (d) never log Authorization headers.'
severity: significant
resolution: 'Three layers: (1) redactKey helper used at every error and log construction site that could see the key; (2) base_url validation rejects user:pass@host URLs (AC #26); (3) AC #24 is a table-driven test that poisons the env var with a sentinel string (SENTINEL_KEY_ZZZZZ) and asserts it appears in NO error or log line across every code path (auth, rate_limited, network, timeout, bad_response, server_error, streaming_unsupported, bad content-type). Operational logging explicitly excludes Authorization headers. CI precommit hook for sk- prefix scanning is deferred to a separate ticket as a generic secret-scanning concern.'
status: addressed
---
