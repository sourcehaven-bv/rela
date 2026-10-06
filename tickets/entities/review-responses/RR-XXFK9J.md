---
id: RR-XXFK9J
type: review-response
title: Attachment upload suffixing leaks other faces' file names
finding: 'Attachment names were unique per entity, so an upload on one face whose name matched a file on another face was suffixed. A writer on a face learned that a face it cannot read holds a file of that name. The coordinator did not accept this as a one-bit channel: a file name is a property value, and entity content on a hidden face is secret. Reopens the BUG-CTUW2N finding.'
severity: significant
resolution: Every upload gets a unique storage key (a random token plus the display name), stored in the value entry. Display names are unique per face only, so an upload on one face behaves as if other faces had no files. Downloads and deletes resolve the name through the face's own value. Copies carry the key and share bytes. Legacy entries keep their base name as key, so no data migration is needed. Pinned by TestFaced_SameNameOnTwoFacesIsIsolated and TestFacedAttachment_UploadRevealsNoOtherFaceName.
status: addressed
---
