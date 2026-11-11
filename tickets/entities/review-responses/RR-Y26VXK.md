---
id: RR-Y26VXK
type: review-response
title: Redirect lookup adds miss-path reads and a timing difference
finding: The redirect called ResolveMentions on every miss, which first re-resolves the missing id through the gated reader (ListEntityHeaders, a second MatchingFaces) and only then asks for successors. This changed the read pattern on every 404 (golden parity failed) and widened the cost difference between misses. A ResolveMentions error was also written as a 500, so TestConflictedRow_RoleRelationsReader got 500 instead of 404.
severity: significant
resolution: 'Added visibility.Resolver.Successor, which asks store.RetiredIDs.Successors for the one id before any gated read and resolves only a found successor. Every miss now costs exactly one extra Successors read (golden diff: Successors=1 added, nothing else, bodies unchanged). A never-retired id does no gated work. Unit test TestResolver_Successor covers live, retired, hidden, deleted, self-mapped and plain-store cases. The conflicted-row test passes again.'
status: addressed
---
