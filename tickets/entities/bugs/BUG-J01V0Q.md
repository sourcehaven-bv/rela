---
id: BUG-J01V0Q
type: bug
title: Write responses name hidden neighbours; entity and relation ETags hash hidden fields
description: Disclosures to a caller who may not see some data. (1) The data-entry create (POST), attachment-upload (PUT _attachments) and history-restore responses render the written entity's outgoing relations from a raw store read. Their relations map therefore names neighbours the caller may not read, for example a secret an on-create automation linked. A hidden entity's existence is a secret (row-level rule), so this discloses entity existence and id. The entity GET gates the same edges through servedFaceEdges + forWireScoped. servedFaceEdges also fell back to a raw ungated read when the neighbour gate was not wired; it now fails closed. (2) The entity ETag (GET, PATCH response and the If-Match check) hashed every property, including visible:-hidden fields, and the single-relation GET ETag hashed hidden relation meta. A caller can guess a hidden low-entropy value offline against the digest, and If-None-Match (304 vs 200) reveals when a hidden value changes. Create, upload and restore set no ETag.
priority: high
effort: s
why1: (1) handleV1CreateEntity, handleV1PutAttachment and writeRestoreResult read edges with reader.outgoingRelations (a raw store query) and serialize them through forWire, which applies no neighbour filter. (2) entityETagWithEdges hashes the entity the visible reader returns, which is row-gated but not field-redacted, and the relation GET hashed the raw relation; redaction happens only in the serializer.
why2: forWire assumes its edges are already gated, and the ETag assumes its entity is already redacted. The neighbour gate lives in servedFaceEdges + forWireScoped and field redaction in stripHiddenProperties, both applied at serialization, so anything computed beside the wire body bypasses them.
why3: 'When the worlds work (#1452) added gated per-entity edges for the GET, the write handlers that render the same v1.Entity shape kept their own edge read; the ETag was written before field-level visible: grants existed and was never revisited.'
why4: Write responses and validators were treated as echoes or checksums of data the caller already has, but automations, pre-existing edges and hidden fields put data there the caller never saw.
why5: Redaction is applied to the wire body rather than to the value every derived output starts from, so each derived output (edges, ETag) must remember to apply it and nothing checks that it does.
prevention: 'Tests pin all of it: internal/dataentry/write_response_neighbors_test.go (create, upload and restore responses omit an unreadable neighbour; servedFaceEdges fails closed when unwired) and internal/dataentry/etag_redaction_test.go (PATCH ETag == GET ETag, neither changes with a hidden value, and the relation ETag ignores hidden meta). Write responses share the GET''s gated edge read; every entity ETag hashes affordanceService.withoutHidden and the relation ETag hashes visibleRelationMeta. Test fixtures wire the neighbour gate as production does.'
started: "2026-10-12"
completed: "2026-10-12"
status: done
---
