---
id: RR-TSBK9Z
type: review-response
title: Nothing enforced that the duplicated filename splitter stays identical to the indexer
finding: The duplication is justified only while exact and there was no mechanism preserving that
severity: significant
resolution: 'Pinned in both directions over the SAME table: TestParseRelationFilename_PinnedForAnalysisCopy in fsstore and TestSplitRelationFilename_MatchesIndexer in analysis. Each test''s doc names the other and says a change to one requires a change to both. Also removed the unreachable extra guard so the two bodies are byte-identical.'
status: addressed
---

`splitRelationFilename` is a deliberate copy of `fsstore.parseRelationFilename`
— unexported there, and arch-lint forbids `analysis -> store/fsstore`, which is
the right boundary. The review endorsed keeping the copy; exporting a storage
detail to save nine lines would be worse.

But the copy is justified *by its exactness* and nothing preserved it. The next
person to fix a parsing bug in fsstore would not know this file exists.

(It had already drifted once: my version carried an extra `relType == ""` guard
the store lacks. Unreachable — `LastIndex(rest, "--") < 1` already rejects
`j==0` — so behavior was identical, but the two no longer *looked* identical,
which is the thing that rots.)

Pinned in both directions: `TestParseRelationFilename_PinnedForAnalysisCopy` in
fsstore and `TestSplitRelationFilename_MatchesIndexer` in analysis, over the
**same table**, covering the cases where they could plausibly drift — all of
which turn on which `--` is chosen. Each test's doc names the other.
