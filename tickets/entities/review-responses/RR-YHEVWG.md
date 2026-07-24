---
id: RR-YHEVWG
type: review-response
title: WaitDelay returns ErrWaitDelay as a spurious command failure on the success path
finding: In cmdexec.execute, when the child exits 0 but a detached grandchild (a pandoc PDF-engine child) holds an output pipe open past waitDelay, Cmd.Wait closes the pipes and returns exec.ErrWaitDelay with ctx.Err()==nil. The code treated any non-nil runErr as a command failure, so a converter that already wrote {out} would intermittently fail with confusing exit noise.
severity: significant
resolution: 'Added an errors.Is(runErr, exec.ErrWaitDelay) carve-out: with ctx not deadline-exceeded, treat it as success and return the captured stdout (Wait has already joined the copier goroutine, so the buffer is stable and the {out} file is on disk). Logged as a warning. Pinned by TestWaitDelayLingeringChildIsSuccessOnOutFile.'
status: addressed
---
