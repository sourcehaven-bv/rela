---
id: RR-Z5ESB3
type: review-response
title: Frontend wire-field test will fail; frontend/src/types/channels.ts is not in scope
finding: TestChannelWireGolden writes channel_wire_fields.json, and frontend/src/types/wireFields.test.ts compares it with CHANNEL_WIRE_FIELDS in frontend/src/types/channels.ts. It requires every Go wire type to be mirrored and the samples to use only mirrored keys. Adding v1.Channel.session, ChannelSession and ChannelSessionTransition in Go without the TypeScript mirror fails npm run test:run in CI. Add channels.ts (field lists and types) to the files to modify in this slice, even though the UI comes in slice 2.
severity: significant
resolution: 'Plan updated: frontend/src/types/channels.ts (types and CHANNEL_WIRE_FIELDS) is in the files to modify; AC 13 requires wireFields.test.ts to pass.'
status: addressed
---
