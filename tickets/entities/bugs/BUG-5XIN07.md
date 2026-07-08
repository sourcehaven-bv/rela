---
id: BUG-5XIN07
type: bug
title: 'attachments: top-level key rejected by metamodel loader'
description: |-
    The `attachments:` config block documented in `docs/attachment-security.md` is unmarshalled correctly by the metamodel struct (`internal/metamodel/types.go` — `Attachments *AttachmentsConfig "yaml:attachments,omitempty"`), but the loader's `checkUnknownKeys` validator rejects it because `attachments` is missing from `validTopLevelKeys` in `internal/metamodel/loader.go:16`.

    Error: `unknown key "attachments" (valid keys: automations, entities, includes, namespace, relations, types, validations, version)`.

    Reproduce: add any valid `attachments:` block per the docs (e.g. `attachments: { scan_cmd: [clamdscan, --no-summary, --fdpass, "{in}"] }`) and run `rela validate` — fails.

    Fix: add `"attachments": true` to `validTopLevelKeys`.

    Introduced in 5574ce01 `feat(attachments): cmd: processing pipeline + native MIME validation (#1026)` — the struct field and the `checkUnknownKeys` validator were both added, but the top-level key whitelist was not updated.

    Impact: any user following the attachment-security docs cannot enable ClamAV scanning or a global MIME allowlist — the whole project fails to load.
priority: high
effort: xs
why1: checkUnknownKeys (internal/metamodel/loader.go) rejected the top-level `attachments:` key because it was absent from the validTopLevelKeys whitelist, even though the Metamodel struct has an Attachments field and the feature is fully wired.
why2: Commit 5574ce01 (#1026) added the Attachments struct field and the checkUnknownKeys validator but did not add the matching `attachments` entry to the validTopLevelKeys whitelist.
why3: validTopLevelKeys is a hand-maintained duplicate of the Metamodel struct's top-level yaml tags; the two are decoupled, so a newly-added field silently drifts out of sync with the validator with nothing binding them.
why4: No test loads a full metamodel with a top-level `attachments:` block through the real Parse()/checkUnknownKeys path. Every attachment test either builds `AttachmentsConfig` as a Go struct literal (internal/attachment/policy_test.go:17) — bypassing the whitelist — or exercises per-property `file` config (which IS whitelisted). There is no CLI test and no Playwright e2e for the scan/allowlist config path. The tests cover AttachmentsConfig *behavior* but never the YAML->struct *wiring*.
why5: Structural allow-list validation (validTopLevelKeys) is a hand-maintained duplicate of the Metamodel struct's top-level yaml tags, with no parity check binding them; and the feature's tests validate behavior via struct literals rather than round-tripping real YAML through the loader, so a wiring gap between a live, documented feature and its config parser is invisible to CI.
prevention: |-
    Root cause: a hand-maintained allow-list (validTopLevelKeys) duplicated the Metamodel struct's top-level yaml tags with nothing binding them, and the attachment feature's tests validated behavior via Go struct literals rather than round-tripping real YAML through the loader — so a new top-level field could be silently rejected by its own parser with no test failing.

    Prevention shipped: added TestValidTopLevelKeysMatchStruct (reflection parity — every top-level yaml tag on Metamodel must be whitelisted), so any future top-level field that isn't whitelisted fails CI at unit level. Added a loader round-trip test and two data-entry HTTP tests that parse a real metamodel with a global attachments block, so the config path is exercised through the actual loader end-to-end.

    Process prevention: filed TKT-ELX09J to apply the same reflection-parity pattern to the two sibling loaders that share this footgun (internal/dataentryconfig/validate.go, internal/acl/policy.go).
status: done
---
