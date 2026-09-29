---
id: RR-VJSEBZ
type: review-response
title: Lua relation writes gated the bare source, not the tail face
finding: '[security] rela.delete_relation and rela.create_relation gated the bare from id, which needs only some readable face, while opts.face could name a face the caller cannot read. The result revealed whether the hidden face holds the edge.'
severity: minor
resolution: relationSource gates the tail address (ID@face) and refuses a fused from or to, so the tail is named only through opts.face. Tests in internal/lua/face_write_test.go (unreadable tail and fused from).
status: addressed
---
