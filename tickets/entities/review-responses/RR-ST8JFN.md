---
id: RR-ST8JFN
type: review-response
title: Attachment fixmes can navigate away mid-upload
finding: The file list can show the file before the PUT completes, and the next navigation may abort it.
severity: significant
resolution: Added FormPage.attachFileAndWaitForUpload, which waits for a successful /_attachments/ PUT; both attachment fixmes use it.
status: addressed
---

The file list can show the file before the PUT completes, and the next
navigation may abort it.
