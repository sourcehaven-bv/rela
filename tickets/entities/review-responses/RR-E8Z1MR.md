---
id: RR-E8Z1MR
type: review-response
title: /_config leaks gated document names, permissions, and script/command strings
finding: 'handleV1Config served the whole dataentryconfig.DocumentConfig map (api_v1.go:1341) plus raw Navigation (1340) to every principal. That defeats the deny path''s uniform 404: rather than probing /_documents/<guess>, a caller reads the document names straight out of _config — along with the `permission:` value naming the grant to seek, the `script:` path, and the `command:` shell string (the last a pre-existing leak). The non-enumerability property was tested at one endpoint, documented in docs/data-entry.md, and recorded in internal/dataentry/CLAUDE.md, while being false at another endpoint in the same file.'
severity: critical
resolution: 'Added a narrow v1.Document wire type (title/entity_type/edit only), following the existing v1.App precedent in the same file, and filtered both Documents and Navigation through a new shared permitsDocument predicate. Verified live: bob (no permission) no longer sees status_review in _config; neither principal sees script paths, permission names, or the command string. Pinned by TestConfig_HidesGatedDocuments, which asserts absence for a non-holder, presence for a holder, and that no execution detail reaches the wire. Frontend DocumentConfig narrowed to match — the typecheck immediately caught a stale test fixture, which is the type doing its job.'
status: addressed
---
