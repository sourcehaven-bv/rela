---
id: RR-XX2LE3
type: review-response
title: Non-string file_name deletes the only file
finding: GetString fell back to "" for a mistyped file_name, turning it into an unnamed delete.
severity: minor
resolution: attachmentArgs uses RequireString when file_name is present. Covered in TestAttachments_ArgumentErrors.
status: addressed
---
