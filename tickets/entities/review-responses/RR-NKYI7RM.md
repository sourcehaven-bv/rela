---
id: RR-NKYI7RM
type: review-response
title: Hook failures fail unrelated writes with a misleading message
finding: A failure to read the author's roles failed the write, and errFlowEngineNotReady did not say why the write needed the engine.
severity: significant
resolution: A role read failure holds no role; a run-list or bind failure fails the write with a named error; the not-ready message names the watching flow.
status: addressed
---
