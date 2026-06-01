---
id: RR-9JOH
type: review-response
title: Dry-run handler skips validateCreateIDOpts; UX divergence with real commit on manual-ID types
finding: 'handleV1DryRunCreate does not call validateCreateIDOpts (the real create does, api_v1.go:477). On a manual-ID type, an id/prefix combo that the real commit will reject (e.g. trimming issues, prefix mismatch) will pass the dry-run silently and only fail at submit. Fix: invoke validateCreateIDOpts in the dry-run too and surface as a soft warning (not 422 - it''s still advisory) so the form learns at typing time.'
severity: minor
resolution: 'Dry-run now invokes validateCreateIDOpts on the trimmed id/prefix and surfaces any failure as a soft warning {code: id_opts_invalid, path: /id} on the response, not a 422 (still advisory). Real commit''s hard rejection unchanged. UX parity: form learns at typing time about id/prefix issues for manual-ID types.'
status: addressed
---
