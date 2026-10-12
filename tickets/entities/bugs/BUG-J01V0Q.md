---
id: BUG-J01V0Q
type: bug
title: Write responses name hidden neighbours; entity ETags hash hidden fields
description: Two disclosures to a caller who may not see some data. (1) The data-entry create (POST), attachment-upload (PUT _attachments) and history-restore responses render the written entity's outgoing relations from a raw store read. Their relations map therefore names neighbours the caller may not read, for example a secret an on-create automation linked. A hidden entity's existence is a secret (row-level rule), so this discloses entity existence and id. The entity GET gates the same edges through servedFaceEdges + forWireScoped. (2) The entity ETag (GET, PATCH response and the If-Match check) hashes every property, including visible:-hidden fields. A caller can guess a hidden low-entropy value offline against the digest. Create, upload and restore set no ETag.
priority: high
effort: s
why1: (1) handleV1CreateEntity, handleV1PutAttachment and writeRestoreResult read edges with reader.outgoingRelations (a raw store query) and serialize them through forWire, which applies no neighbour filter. (2) entityETagWithEdges hashes the entity the visible reader returns, which is row-gated but not field-redacted; redaction happens only in the serializer.
why2: forWire assumes its edges are already gated, and the ETag assumes its entity is already redacted. The neighbour gate lives in servedFaceEdges + forWireScoped and field redaction in stripHiddenProperties, both applied at serialization, so anything computed beside the wire body bypasses them.
why3: 'When the worlds work (#1452) added gated per-entity edges for the GET, the write handlers that render the same v1.Entity shape kept their own edge read; the ETag was written before field-level visible: grants existed and was never revisited.'
why4: Write responses and validators were treated as echoes or checksums of data the caller already has, but automations, pre-existing edges and hidden fields put data there the caller never saw.
why5: Redaction is applied to the wire body rather than to the value every derived output starts from, so each derived output (edges, ETag) must remember to apply it and nothing checks that it does.
prevention: 'Tests pin both: internal/dataentry/write_response_neighbors_test.go (create, upload and restore responses omit an unreadable neighbour) and internal/dataentry/etag_redaction_test.go (PATCH ETag == GET ETag and neither changes with a hidden value). Write responses share the GET''s gated edge read; every entity ETag hashes affordanceService.withoutHidden.'
started: "2026-10-12"
completed: "2026-10-12"
status: done
---
