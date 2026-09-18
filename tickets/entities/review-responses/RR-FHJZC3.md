---
id: RR-FHJZC3
type: review-response
title: Stale reference to a deleted module in a comment
finding: entityRefIdGrammar.test.ts cited 'same pattern as markdownContentMirror', a test file this change
  deletes. The repo runs a blocking doclink comment gate, so dangling prose references are worth clearing
  as they appear.
severity: nit
resolution: Repointed at serializerContract.test.ts, which uses the same file-local node-types trick and
  is not going anywhere.
status: addressed
---
