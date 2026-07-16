---
id: RR-1WTST
type: review-response
title: Self-loop edge reported performable on read but is a no-op (unenforced) on write
finding: 'A transition edge with From==To compiles fine but is never enforced on write: EnforceUpdate skips unchanged values (from==to -> continue) before reaching the edge. Performable iterated all edges with key.from==from including key.to==from, reporting the self-loop as a performable transition (running its guard/precondition). Read claims an action the write path never enforces -> divergence.'
severity: critical
resolution: Performable now skips key.to==from (self-loop) with a WHY comment tying it to the write-path no-op skip. Parity test's driftMeta includes an a->a self-loop and asserts BOTH that Performable omits it AND that the corresponding write is a no-op (allowed), so the omission stays correct.
status: addressed
---

## Finding

`From == To` compiles cleanly but is a no-op on write (`EnforceUpdate` skips
`from == to` before reaching the edge). `Performable` reported it as a
performable transition — claiming an action the write path never enforces.

## Fix

`Performable` skips `key.to == from`. Parity test's `driftMeta` has an `a→a`
self-loop; the test asserts it's omitted from verdicts AND is a no-op on write.
