---
id: RR-0RLYP
type: review-response
title: 'Mock leakage risk: clearAllMocks vs resetAllMocks'
finding: vi.clearAllMocks() resets call history but not mockRejectedValueOnce/mockResolvedValueOnce queues or future mockReturnValue defaults. Tests are safe today, but switching to vi.resetAllMocks() in beforeEach is cheap forward-compat insurance.
severity: nit
resolution: Switched useListActions.test.ts beforeEach from vi.clearAllMocks() to vi.resetAllMocks() so future mockReturnValue defaults can't bleed across cases. Added a brief comment explaining the rationale.
status: addressed
---
