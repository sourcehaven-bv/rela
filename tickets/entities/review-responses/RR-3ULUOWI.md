---
id: RR-3ULUOWI
type: review-response
title: Upload slot taken before the post is authorized
finding: The HTTP multipart post took an upload slot before checking that the caller may post, so a reader without post rights could hold slots.
severity: significant
resolution: FileOpener.CheckUpload runs the read check, post roles, open state and file-property check after the message part and before the slot. Tested in TestChannelFiles_BusyUploads and TestCheckUpload.
status: addressed
---
