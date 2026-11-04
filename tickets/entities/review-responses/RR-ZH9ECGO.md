---
id: RR-ZH9ECGO
type: review-response
title: No test pins that a hidden peer's type does not leak
finding: Every type-contract case used readable peers; nothing enforced that a hidden disallowed peer is answered like a missing one.
severity: significant
resolution: 'TestRelationWrite_HiddenDisallowedPeerAnswersAsMissing compares a hidden tag peer with a missing id on PATCH (canonical and inverse), POST and the incoming per-edge POST: same status and body, no type code, nothing written or audited.'
status: addressed
---
