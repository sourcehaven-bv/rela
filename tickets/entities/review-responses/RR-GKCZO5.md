---
id: RR-GKCZO5
type: review-response
title: Fail-open on gate construction error is wrong for the scheduler path specifically
finding: 'scriptEntityReader/scriptTracer return the RAW store on any construction failure (six branches, all returning st, with a slog.Warn). For dataentry that is defensible — a broken gate breaking every request is a worse outage and the warning is seen. For the SCHEDULER it is not: the stated guarantee is ''what never enters the reader never enters a prompt'', and a slog.Warn in a nightly batch job is read by nobody until after the prompt has been built and sent to a third-party model. The failure mode is unbounded, silent and irreversible (exfiltration), not an outage. There is also no operator control to choose deny. Given DEC-ZBI39P''s fail-closed posture elsewhere (permittedIDs drops a type fail-closed on the same error class), the asymmetry needs either a recorded justification or a fail-closed scheduler path.'
severity: significant
resolution: 'Agreed for the unattended paths and fixed there: added visibility.DenyReader / DenyTracer, and appbuild''s scriptEntityReader/scriptTracer now REFUSE (returning them, logged at Error) when a policy IS configured but the gate cannot be built — these deps back automation cascades and scheduled tasks, where silently reverting to full-graph reads is an unbounded irreversible disclosure rather than an outage. Data-entry deliberately keeps the permissive fallback: it is interactive, the operator sees the failure immediately, and breaking every request is the worse and louder outage. The asymmetry is now explicit and documented at both call sites rather than incidental.'
status: addressed
---
