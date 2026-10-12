---
id: BUG-J01V0Q
type: bug
title: Write responses name neighbours the caller cannot read
description: The data-entry create (POST), attachment-upload (PUT _attachments) and history-restore responses render the written entity's outgoing relations from a raw store read. Their relations map therefore names neighbours the caller may not read, for example a secret an on-create automation linked. A hidden entity's existence is a secret (row-level rule), so this discloses entity existence and id. The entity GET gates the same edges through servedFaceEdges + forWireScoped.
priority: high
effort: s
why1: handleV1CreateEntity, handleV1PutAttachment and writeRestoreResult read edges with reader.outgoingRelations (a raw store query) and serialize them through forWire, which applies no neighbour filter.
why2: forWire assumes its edges are already gated; the neighbour gate lives in servedFaceEdges + forWireScoped, and only read paths were moved onto it.
why3: When the worlds work (#1452) added gated per-entity edges for the GET, the write handlers that render the same v1.Entity shape kept their own edge read instead of sharing the GET's.
why4: Write responses were treated as echoes of the caller's own write, but automations and pre-existing edges put neighbours there that the caller never named.
why5: The serializer offers an ungated entry point (forWire) beside the gated one, so every new caller must choose correctly and nothing checks the choice.
prevention: Tests in internal/dataentry/write_response_neighbors_test.go pin that the create, upload and restore responses omit a neighbour the caller cannot read. Write responses now share the GET's gated edge read (servedFaceEdges + forWireScoped).
started: "2026-10-12"
completed: "2026-10-12"
status: done
---
