---
id: RR-8K0AEM
type: review-response
title: Lost-race 412 and PATCH tokens re-read the row without the row gate
finding: '[security] writeLostRace and storedVersions re-read via the ungated reader. A concurrent write that hides the row from the caller (owner change) still yields a 412/200 with tokens of the new state; 8-hex tokens of low-entropy values are brute-forceable. A GET would 404.'
severity: significant
resolution: writeLostRace and the PATCH success path now re-read through visibleReader.getVisibleRef (new writeHandler.readVisible / visibleStored). A row that is gone, retyped or hidden yields the ordinary 412 without versions, or a 200 without _versions. The success body and tokens now come from the same gated read. Pinned by TestV1UpdateEntity_LostRaceToHidingWriteCarriesNoTokens and TestV1UpdateEntity_WriteThatHidesRowCarriesNoTokens (both mutation-verified).
status: addressed
---
