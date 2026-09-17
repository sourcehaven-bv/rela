---
id: RR-XN8DTF
type: review-response
title: Sampled corpus run looked like the full gate locally
finding: serializerContract.test.ts defaults to STRIDE=8, sweeping one eighth of the corpus, with the full sweep only under RELA_FULL_CORPUS=1. CI sets it, so the gate exists — but a developer changing RELA_STRINGIFY_OPTIONS locally sees green over 493 of 3,942 files and finds out in CI. Given those options decide how every stored file gets rewritten, a sampled pass should not read as clearance.
severity: significant
resolution: The sampled path now emits a console.warn saying it sampled 1-in-N, that this is NOT the full gate, and naming the command to run before changing RELA_STRINGIFY_OPTIONS or semanticShape. Kept as a warning rather than making the full sweep the default, because a 64-second unit-test run would be worse.
status: addressed
---

## Finding

`STRIDE = 8` by default. The existing log line printed `493/3942`, which is
accurate but easy to read past in a wall of passing tests.

The risk is specific: `RELA_STRINGIFY_OPTIONS` determines how 3,939 stored files
are rewritten. A developer tuning it, seeing green, and pushing has done one
eighth of the verification the change warrants.

## Resolution

The sampled path warns explicitly:

```text
corpus: SAMPLED 1-in-8. This is NOT the full gate — run
RELA_FULL_CORPUS=1 npm run test:run before changing
RELA_STRINGIFY_OPTIONS or semanticShape. CI runs the full sweep.
```

Deliberately not making the full sweep the default: it takes about 64 seconds,
which is too slow for the run developers do constantly, and slow tests get
skipped. The warning targets the specific moment the sample is insufficient.
