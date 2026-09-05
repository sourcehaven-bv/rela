---
id: RR-FSVDOC
type: review-response
title: Doclinks to unimported packages, a redundant errors.Is pair, and a one-sentinel errors.go
severity: minor
status: addressed
finding: 'Three small things. (1) The doc referenced [datamigration.LoadDir] and [io/fs.ReadFileFS]; Go cannot resolve a link to an unimported package, and io/fs is imported as fs, so both rendered as literal brackets — and doclink is a blocking commentlint gate. (2) Open checked both errors.Is(err, fs.ErrNotExist) and errors.Is(err, os.ErrNotExist), which are the same variable (os.ErrNotExist = fs.ErrNotExist), suggesting the author was unsure and leaving the next reader unsure too. (3) internal/config/errors.go held a single unexported sentinel used once, forty lines away, while every other error in the package is constructed inline.'
resolution: '(1) Unresolvable links unbracketed, io/fs ones written as [fs.X]; just comment-lint clean. (2) The redundant pair is gone with the Open rewrite (RR-FSVOPEN). (3) errors.go deleted, the sentinel inlined at NewFSView. Also fixed a govet shadow in the new test.'
---
