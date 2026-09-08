---
id: RR-FSVSIZE
type: review-response
title: DirEntry.Info reported Size 0 for every file, fabricating the detail the ModTime comment refused to
severity: significant
status: addressed
finding: 'memDirEntry.Info returned a FileInfo with no size set, so every file listed through ReadDir reported Size() == 0 — a wrong number, not an absent one. The adapter had already got this principle right one type up: ModTime deliberately returns the zero time with a comment explaining that a fabricated timestamp is worse than an obviously absent one, because a consumer comparing it would silently treat every read as freshly modified. Size 0 is the same trap, and the comment on the neighbouring line claimed the code was avoiding it.'
resolution: 'Info now returns an error stating the metadata is unavailable from a listing. Loader.List returns names only, so a size genuinely cannot be known without reading the file, and an error is the honest way to say so. Pinned by TestFSView_DirEntryInfoReportsNoSize.'
---
