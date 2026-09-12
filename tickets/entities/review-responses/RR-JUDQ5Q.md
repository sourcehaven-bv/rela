---
id: RR-JUDQ5Q
type: review-response
title: decideEmit ignored a revert, so the form kept an intermediate value
finding: 'decideEmit compared the guarded result against `original` to decide whether there was anything to tell the parent. A user who types and then deletes what they typed produces markdown equal to the original again, so the emit was skipped — but the parent still held the intermediate value it had already been told about, and would save that. Verified: decideEmit(''hello\n'',''hello\n'',''hello\n'',true) returned {action:''ignore''}.'
severity: significant
resolution: Threaded lastEmitted through decideEmit. The comparison is now against what the parent actually holds, falling back to `original` only before anything has been emitted. The early markdown===settled shortcut is likewise gated on nothing having been emitted yet, since a revert lands back on the settled value.
status: addressed
---

## Finding

`decideEmit` asked "is the guarded value different from the original?" when the
question is "is it different from what the parent currently holds?" Those
diverge as soon as anything has been emitted.

Sequence: open `hello`, type ` world` (parent told `hello world`), delete it
again. The markdown is `hello` once more, equal to `original`, so the emit was
skipped and the parent kept `hello world`. Saving stored the edit the user had
just undone.

## Resolution

`lastEmitted` threaded through, reset on reload alongside the other baselines.
The comparison uses it when present and `original` before the first emit.

The `markdown === settled` shortcut needed the same treatment: a revert lands
exactly on the settled value, so that check now applies only while nothing has
been emitted.

Two tests: one asserts the revert is emitted when the parent holds something
else, one asserts a repeat of what the parent already holds is ignored.
