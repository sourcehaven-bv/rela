---
id: RR-KSN4XU
type: review-response
title: The error mapping does not match the server's contract
finding: '§4 and §5 map 422 to "field errors from the problem details". In channels_errors.go, prop validation is 400 (TypeInvalid, TypeTooManyProps, TypeReasonRequired), size is 413 (TypeTooLarge), 422 is only TypeInvalidRef, TypePromoteInvalid, TypeSignalsNotKept and TypeFaceRequired, and the body carries type, title and detail (the rule), never a field name. Also, "404 on a channel route drops held content" would wipe the panel when one message was purged during an action (TypeMessageNotFound), and it misses TypeChannelNotFound (channel deleted) versus the anchor''s uniform 404. Fix: map by problem `type`, not status. 400/413/422 go to a composer banner with the title (field highlight only where the client''s own check knows it). A message-level 404 refetches. TypeChannelNotFound returns to the list. Only a refused head or list refetch drops content. Add a table test in ChannelsPanel.test.ts per type.'
severity: significant
resolution: Errors are mapped by problem type per channels_errors.go, message-level errors refetch without clearing the panel, and only a refused head or list refetch drops content (R12, AC 14).
status: addressed
---
