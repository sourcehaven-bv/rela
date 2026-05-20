---
id: RR-LPAV
type: review-response
title: MCP analyze_validations reports 'all rules passed' while silently skipping encrypted entities
finding: validator.GenericValidator.loadCandidates skips inaccessible entities silently — no count, no warning, no log surfaced to the caller. The MCP analyze_validations tool then reports 'All N rules passed' and the Claude-agent-driven ticket-done workflow marks the ticket done. The human believes encrypted entities were validated; they were not. Add SkippedInaccessibleIDs (or a count) to validator.RuleResult and surface it in the MCP response and the data-entry Analyze view. Without this, the validator skip is a silent failure mode masquerading as a green light.
severity: significant
status: deferred
reason: |-
    Parent ticket TKT-PGK91 (git-crypt detection) shipped via PR #668 without addressing this finding. Captured here so the gap remains visible; will be revisited if the underlying code path becomes a problem in practice. Closed as deferred via the TKT-5S8T data-debt sweep — the alternative is leaving the RR open indefinitely while it blocks every unrelated PR.
---
