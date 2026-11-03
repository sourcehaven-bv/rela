---
id: RR-D8VSSBU
type: review-response
title: Rename input ignores update permission
finding: A reader without update rights on a group saw an editable heading.
severity: minor
resolution: 'renamable(groupId) also requires update on the group entity; otherwise the name is plain text. Test added. Group moves stay as they were: they write the page entity''s edges, whose _actions the board does not load; the server refuses and the error shows.'
status: addressed
---
