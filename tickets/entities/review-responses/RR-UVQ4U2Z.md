---
id: RR-UVQ4U2Z
type: review-response
title: Fake snapshot source ambiguous
finding: The plan says globalSetup exports the schema and config as the fake's snapshot, but property tests run in the normal suite with no server. If the snapshot is regenerated per run, the unit suite has none; if it is a copy nobody checks, it drifts where the differential test does not look.
severity: significant
resolution: The fixture project and the schema/config snapshot are committed files under frontend/contract/fixture. The contract test asserts the live server's responses equal the committed snapshot.
status: addressed
---
