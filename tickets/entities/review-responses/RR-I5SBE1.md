---
id: RR-I5SBE1
type: review-response
title: on_final close runs on the client's cancellable context
finding: 'internal/realtime/outputs.go:687-691: close() passes ctx straight to applyTransition. Inline, ctx is c.ctx (service.go runInline -> s.outputs.run(c.ctx, ...)). A client that disconnects after the outputs gets a context-canceled error, the result is recorded as failed/error, and the done write (which does use WithoutCancel) makes it final, so the channel never closes. Writes and progress use context.WithoutCancel plus a timeout; close must do the same. Fix: wrap it as write() does: context.WithTimeout(context.WithoutCancel(ctx), markTimeout). Add a test that cancels the ctx before the close.'
severity: significant
resolution: on_final close and output writes run on WithoutCancel with their own timeouts; TestOutputs_CloseDetachedFromClient.
status: addressed
---
