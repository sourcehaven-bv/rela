---
id: RR-NA8DML
type: review-response
title: Event UID scheme <type>-<id> is ambiguous for hyphenated entity types
finding: 'splitFeedUID (feed_uid.go:27) splits the local part with strings.Cut(local, "-") — the FIRST hyphen. But entity types AND ids both contain hyphens (metamodel has test-case, test-suite, doc-task, review-response; ids like TSK-1). feedUID("test-case","TC-1") = "test-case-TC-1@rela"; splitFeedUID returns ("test","case-TC-1") — wrong. The <type>-<id> scheme is fundamentally ambiguous with ''-'' as delimiter; there is no correct split, so it''s a UID-design defect. Latent today (Get/CalDAV unrouted) but ships with TestSplitFeedUID that only tests task/party, masking it. When CalDAV lands, per-resource fetch for hyphenated types 404s and Get''s source-routing silently never matches. Fix NOW before the wire format is pinned: use an unambiguous delimiter (entity ids forbid ''/'' and ''--''), and add a hyphenated-type test case.'
severity: significant
resolution: Changed the UID separator from '-' to '--' (feedUIDSep). Entity ids reject '--' (it's the relation-key separator, per entity.ValidateID) and type names are single-hyphen kebab, so '--' appears in neither — the split is now unambiguous even for hyphenated types. Pinned by TestSplitFeedUID with test-case/review-response/doc-task round-trip cases + a single-hyphen-rejected case.
status: addressed
---
