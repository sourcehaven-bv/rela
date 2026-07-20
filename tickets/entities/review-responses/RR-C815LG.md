---
id: RR-C815LG
type: review-response
title: 'IB-review: 400 on invalid filter operators dropped the server-side slog.Warn (CONTROL-8-15 / RR-6RF60V logging intent)'
finding: PR #1171 replaced the slog.Warn-and-degrade handling of malformed/unknown filter operators with a hard HTTP 400 — but the errBadFilter branch of writeListPipelineError was the only branch WITHOUT a server-side log. RR-6RF60V (critical, addressed) had explicitly required entities[:0] + slog.Warn on unknown relation-filter operators, and CONTROL-8-15 expects errors/anomalous events to be logged server-side. Raised by tschmits on PR #1171.
severity: significant
resolution: 'Deliberate to replace the DEGRADE with a 400; NOT deliberate to drop the log. Restored: the errBadFilter branch now does slog.Warn("dataentry: rejected invalid filter parameter", err, path, method) before writing the 400, matching the other writeListPipelineError branches. RR-6RF60V''s fail-closed intent is preserved (the 400 never falls open) and its logging intent plus CONTROL-8-15 coverage are restored — the response informs the caller, the log informs operators (broken configs and filter-param probing leave a trace).'
status: addressed
---
