---
id: RR-HQ25E2
type: review-response
title: CaptureLogs test assumes the stdlib default slog handler at start
finding: TestCaptureLogs_RestoresLogOutput relies on slog.Default being the default handler when it starts.
severity: nit
resolution: Not changed.
reason: True for the testutil test binary, which has no init that changes the default logger.
status: wont-fix
---
