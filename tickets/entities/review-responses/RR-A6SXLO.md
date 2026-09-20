---
id: RR-A6SXLO
type: review-response
title: Corpus serializer test does not exercise the relaComment node
finding: '`serializerContract.test.ts` builds a bare `unified().use(remarkParse).use(remarkGfm).use(remarkStringify)` processor with no ProseMirror schema, so the ~3,939-file corpus gate round-trips markdown→mdast→markdown and never exercises relaComment. The corpus full of template comments is not protecting this feature.'
severity: significant
reason: 'Correct as stated, and deliberately not fixed in this ticket. The claim is about the corpus harness generally, not about this node: NO editor node is covered by it, including entityRef and taskList, because the harness deliberately tests the serializer contract rather than the schema. Extending it to drive markdown through a mounted editor is a change to shared test infrastructure affecting every node, with real cost (mounting an editor per corpus file). Filed as follow-up work rather than bolted on here. The immediate risk is covered: RR-2UZZAO''s verdict assertions now fail on a corrupting serializer, and the app-editor round-trip test (RR-XBKAGN) exercises the real save path on both editors.'
status: deferred
---

The reviewer framed this as "with #1 fixed you have unit tests; with #1 unfixed
you have nothing". #1 is fixed and mutation-verified, so the gap that remains is
the absence of a corpus-wide gate — real, but pre-existing and not specific to
this change.

Follow-up shape, if taken: a corpus test that mounts one editor and feeds each
file through `serialize()`, comparing bytes directly rather than through
`guardWriteBack` — the guard is precisely what launders the failure.
