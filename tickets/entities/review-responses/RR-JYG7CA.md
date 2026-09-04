---
id: RR-JYG7CA
type: review-response
title: Real-fsnotify regression test could pass because the self-write was never observed, not because it was suppressed
finding: TestFSFactoryWatcherSuppressesSelfEcho only pre-created entities/ and relations/. SafeFS.WriteFile MkdirAll's entities/policies/ during the first write, and the watcher registers a new subdirectory only on its Create event, so POL-1's own file event could slip through unwatched — a green result for the wrong reason that would stay green after a regression.
severity: significant
resolution: 'The test pre-creates the entities/policies leaf so the self-write is genuinely observed by fsnotify and then suppressed. Verified: with the leaf pre-created the test fails on develop''s code (duplicate EntityPut and spurious Updated) and passes 10/10 with -race on the fix.'
status: addressed
---
