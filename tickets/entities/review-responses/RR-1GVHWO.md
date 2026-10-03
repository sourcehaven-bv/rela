---
id: RR-1GVHWO
type: review-response
title: StampAttachments accepts a non-file property
finding: The privileged stamp did not check that the named property is a file property of the stored type.
severity: minor
resolution: rejectFilePatch refuses an allowed property that is not a file property. Pinned by TestStampAttachments_RefusesANonFileProperty.
status: addressed
---
