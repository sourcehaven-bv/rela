---
id: RR-O0KRI2
type: review-response
title: clear_when_hidden silently ignored by old binaries — nested config keys are never validated
finding: 'checkUnknownKeys (internal/dataentryconfig/validate.go:150-168) only walks TOP-LEVEL keys of the raw map; nested forms[].fields[] keys are never checked, and the struct unmarshal is plain non-strict yaml.Unmarshal (no KnownFields(true) anywhere in the package). An unknown nested key decodes with err=nil. Failure scenario: an operator adds clear_when_hidden: no to opt out of clearing, but a node still running an older binary parses the config cleanly, reports no error, and applies the old hardcoded destructive behavior — the exact BUG-FB0LN8 data loss, now with the operator believing they have opted out. The design is silent on version-gating a key whose absence means ''destroy data''.'
severity: significant
resolution: |-
    WON'T FIX in this ticket — accepted risk, documented reasoning.

    The finding is factually correct: checkUnknownKeys only walks TOP-LEVEL config keys, nested form-field keys are never structurally validated, and the struct unmarshal is non-strict. An old binary reading a new config with clear_when_hidden silently ignores it.

    Why not fixed here:

    1. It cannot be fixed FROM this change. A new binary cannot make an OLD binary reject a key the old binary has never heard of. Any gate would have to have shipped in the previous release. That is the general shape of every additive config key in this codebase — clear_when_hidden is not special.

    2. The blast radius is bounded and shrinking in the safe direction. The affected case is an operator who explicitly writes clear_when_hidden: no (or confirm) to opt OUT of clearing while running a binary that predates the key. On such a binary the pre-fix destructive behavior applies — i.e. they are exactly as badly off as before the fix, not worse. After upgrading, the NEW default is already non-destructive without any config, so the common path needs no opt-in at all.

    3. A general fix belongs in its own ticket: adding recursive unknown-key validation for nested config structures (forms[].fields[], lists[].columns[], views[].sections[], ...). That is a broad change with its own compatibility surface — it would start rejecting configs that parse cleanly today — and folding it into a critical data-loss fix would delay the fix and enlarge its risk.

    Mitigation shipped instead: the release note calls out that the destructive behavior is now opt-in and that clear_when_hidden requires a binary from this release or later.
reason: |-
    Cannot be fixed from this change: a new binary cannot make an OLD binary reject a key it has never heard of — any such gate would have had to ship in the previous release. This is the general shape of every additive config key in this codebase, not something specific to clear_when_hidden.

    Blast radius is bounded and fails in the safe direction: the affected case is an operator who explicitly writes clear_when_hidden to opt OUT of clearing while running a pre-release binary. On that binary they get the old destructive behavior — exactly as badly off as before this fix, not worse. After upgrading, the new default is already non-destructive with no config at all, so the common path needs no opt-in.

    The general fix (recursive unknown-key validation for nested config structures) is filed as TKT-QHF4JQ. It would start rejecting configs that parse cleanly today, which is its own compatibility surface and deserves its own review rather than riding along with a critical data-loss fix. Mitigation shipped instead: the release note states the destructive behavior is now opt-in and that clear_when_hidden requires a binary from this release or later.
status: wont-fix
---
