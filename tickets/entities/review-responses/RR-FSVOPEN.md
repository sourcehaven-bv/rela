---
id: RR-FSVOPEN
type: review-response
title: FSView.Open returned a successful empty directory for a missing file
severity: critical
status: addressed
finding: 'Open fell back to a directory listing when Load reported not-found. Because Loader.List reports an ABSENT directory as an empty list, that fallback could not distinguish "empty directory" from "no such path" — so Open("migrations/typo.yaml") returned a successful empty directory handle instead of an ErrNotExist. That violates the fs.FS contract ("Open should return an error satisfying fs.ErrNotExist" for a nonexistent path). It was masked only because FSView implements ReadFileFS, so fs.ReadFile never fell back to Open; anything wrapping FSView in a way that loses that assertion would have turned every missing-file read into a bogus empty directory.'
resolution: 'Directory handling is now restricted to the root ("."), which is the only case that was ever justified — fs.WalkDir and fs.Glob open the root before anything else, and an fs.FS whose root cannot be opened is malformed. Every other name must resolve to a file, so a missing path returns the Loader''s ErrNotExist unchanged. Verified before and after: Open of a missing file previously returned a non-nil handle with a nil error, and now returns isNotExist=true. Pinned by TestFSView_MissingFileIsNotExist, which also covers the fs.ReadFile fallback path.'
---
