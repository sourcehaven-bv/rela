---
id: DOCS-HVPLAAL
type: docs-checklist
title: 'Docs: Attachments in channel messages'
status: done
---

## Code Documentation

- [x] Comments where logic isn't obvious (file staging and trash order in filechannels/files.go; preflight order in dataentry/channels_files.go; runner holder in appbuild/realtime_files.go)
- [x] Function/type docs if public API (FileOpener, CheckUpload, FilePropertyView, SetChannelFileRunner, MaxFile* limits)

## Project Documentation

- [x] ~~README updated~~ (N/A: no README content covers channel message types)
- [x] ~~CLAUDE.md updated~~ (N/A: no new cross-cutting pattern; the runner holder follows the existing attachment rules)
- [x] Help text accurate (rela channel post --file, rela channel file --to)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repository keeps no changelog file; release notes come from PR titles)
- [x] API docs updated (GUIDE-realtime: file properties, multipart post, download route and errors; GUIDE-cli-reference: --file and channel file; GUIDE-mcp-server: post_message files and read_message_file; docs/ regenerated)
