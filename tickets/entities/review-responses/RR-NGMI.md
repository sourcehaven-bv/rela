---
id: RR-NGMI
type: review-response
title: 'Timing oracle: hidden id costs ~1ms (GraphCount), nonexistent costs ~1µs (no gate call)'
finding: 'In handleV1GetEntity: nonexistent id → getEntity returns (nil, false) → immediate 404 (no gate call, no DB roundtrip). Hidden id → getEntity returns (entity, true) → gate.Visible → GraphCount → 404. Order-of-magnitude timing difference enumerates the id-space by timing alone — defeats the doc''s threat-model commitment (''An attacker who can probe URLs sees only 404 for every hidden entity''). Fix: run the gate BEFORE getEntity. The gate only needs type+id from the URL, not the entity object. Both nonexistent and hidden paths then spend the same GraphCount roundtrip. Apply the same swap to Update/Delete/CloneEntity for consistency.'
severity: significant
resolution: Visible probe moved BEFORE getEntity in handleV1GetEntity / Update / Delete / Clone. Hidden and nonexistent both spend the same GraphCount roundtrip.
status: addressed
---
