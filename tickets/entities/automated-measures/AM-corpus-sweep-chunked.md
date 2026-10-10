---
id: AM-corpus-sweep-chunked
type: automated-measure
title: Corpus round-trip runs in bounded chunks
description: serializerContract.test.ts splits the full corpus sweep into tests of at most 250 files per directory, each with its own 120 s timeout, so corpus growth adds tests instead of pushing one test past its cap (BUG-L6EDB8).
kind: test
location: frontend/src/components/forms/milkdown/serializerContract.test.ts
status: active
---
