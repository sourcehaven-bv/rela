---
id: RR-JIRLLU
type: review-response
title: 'Missing tests: content-cap reader path, idempotent delete; misleading delete comment; cleanup nits'
finding: 'S1: no test where header.Size under-declares and limitedContentReader.exceeded trips (dead-untested branch). S5: no idempotent-delete test (204 + property empty when no attachment); delete comment lists ''deny'' as an orphan cause but deny is handled in preflight before UpdateEntity. N1: handleV1DeleteAttachment threads a dead `plural` param (''_ = plural''). N3: maxAttachmentBytes() does its own a.State() load instead of snapshotting once. N2: multipart temp files not explicitly RemoveAll''d. S3/L1: two parallel limit-reader implementations (limitedContentReader + storeutil.attachmentLimitReader) with different off-by-one styles — consolidate.'
severity: minor
resolution: 'S1: the content-cap path is now the shared store.CapAttachmentReader, covered by store-package unit tests (TestCapAttachmentReader: at-limit, over-limit, unbounded, zero) + storetest oversize cases. S5: added TestAttachmentDelete_IdempotentWhenNoAttachment (204 + property empty); corrected the delete doc comment (deny is handled in preflight, only a validation failure orphans bytes). N1: dropped the dead `plural` param from handleV1DeleteAttachment + its dispatch. N3: handler snapshots a.State() once and maxAttachmentBytes takes *AppState. N2: added a defer r.MultipartForm.RemoveAll(). S3/L1: consolidated the two limit-readers into a single store.CapAttachmentReader(r, limit) — storeutil.LimitAttachmentReader and the handler both delegate to it; removed the duplicate limitedContentReader. (Reader lives in internal/store, not storeutil, to respect the dataentry→store arch boundary.)'
status: addressed
---
