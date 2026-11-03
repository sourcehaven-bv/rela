---
id: RR-20GTXR
type: review-response
title: Store ErrNotFound becomes 500
finding: A channel or message removed between open and Append/Edit/Remove passes through storeError as a raw error.
severity: minor
resolution: channels.ErrNotFound maps to channel_not_found, or message_not_found on message operations.
status: addressed
---
