---
id: RR-FREVJV6
type: review-response
title: Type-label heading follows attached types, not readable ones
finding: ChannelsPanel decides that the page has two blocks from the channel types realtime.yaml attaches. A reader who cannot read the placed type sees one block headed with the other type's label, while the docs said the label is used when a page has two blocks.
severity: minor
resolution: Docs (data-entry.md and its guide mirror) and the ChannelsPanel doc comment now say the heading follows the configuration, not what the reader may read.
status: addressed
---
