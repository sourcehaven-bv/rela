---
id: RR-FSVLIST
type: review-response
title: ReadDir had no coverage of its error path, and no contract for a Loader that reports absence as ErrNotExist
severity: critical
status: addressed
finding: 'FSView.ReadDir forwarded Loader.List errors verbatim. datamigration.LoadDir treats ONLY errors.Is(err, fs.ErrNotExist) as "empty chain" and surfaces everything else, deliberately — an unreadable migrations/ reported as "nothing to migrate" would let a chain silently go unrun. With FSLoader the outcomes happened to be preserved, but by two mechanisms agreeing rather than by design: FSLoader.List returns (nil, nil) for an absent directory, so LoadDir''s ErrNotExist branch became dead code. A second Loader — the SQLite one this feature exists for — would have to independently rediscover that convention, and returning ErrNotExist instead (the obvious choice, and what Load is documented to do) would have failed the whole migration chain on any project with no migrations. There was also zero test coverage of ReadDir''s error path: every mapLoader.List in the suite returned a nil error unconditionally.'
resolution: 'ReadDir now normalizes: an os.ErrNotExist-compatible error from List means "no such directory" and lists empty, matching FSLoader''s empty-list spelling; every other error surfaces. The contract is stated on the method. Two tests added — TestFSView_ReadDir_SurfacesLoaderErrors (a real error must not be flattened) and TestFSView_ReadDir_NotExistFromListListsEmpty (the alternative spelling is accepted).'
---
