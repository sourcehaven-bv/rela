---
id: RR-H6L1JIJ
type: review-response
title: Restart refills empty rooms with old documents
finding: ygo Shutdown closes sockets without a 44xx code, so clients reconnect within seconds and push their documents into fresh rooms, where they can meet a document a new client seeded from the store. pruneSeeds then deletes one copy with its edits, or both copies survive as duplicated text.
severity: significant
resolution: 'The relay has a per-process epoch sent in _config, the SPA joins with it and an earlier epoch is closed with 4410, on which the client saves what it holds and closes the editor. Tests: TestCollab_EarlierEpochIsClosed, collabRoom 4410 test.'
status: addressed
---
